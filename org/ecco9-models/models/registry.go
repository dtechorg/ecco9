// Package models implements the model registry and conversion lifecycle:
// pull/push/list/delete/convert, content-addressed blob management, and an
// object-storage abstraction with a local filesystem implementation.
// Adapted from server/create.go, server/download.go, server/upload.go,
// server/images.go, convert/convert.go, and fs/gguf/reader.go.
package models

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"sync"
	"time"
)

// ModelInfo mirrors the proto ModelInfo message.
type ModelInfo struct {
	Name           string            `json:"name"`
	Digest         string            `json:"digest"`
	SizeBytes      int64             `json:"size_bytes"`
	Format         string            `json:"format"` // gguf, safetensors, ...
	Labels         map[string]string `json:"labels,omitempty"`
	ModifiedAtUnix int64             `json:"modified_at_unix_ms"`
}

// Layer is one content-addressed blob referenced by a manifest, adapted
// from server/images.go's Layer.
type Layer struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

// Manifest ties a model name to its layers, adapted from server Manifest.
type Manifest struct {
	Name    string            `json:"name"`
	Config  Layer             `json:"config"`
	Layers  []Layer           `json:"layers"`
	Labels  map[string]string `json:"labels,omitempty"`
	Created time.Time         `json:"created"`
}

// Digest computes the manifest digest over its canonical layer list.
func (m *Manifest) Digest() string {
	h := sha256.New()
	_, _ = io.WriteString(h, m.Name)
	_, _ = io.WriteString(h, m.Config.Digest)
	for _, l := range m.Layers {
		_, _ = io.WriteString(h, l.Digest)
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}

// GetSHA256Digest streams r, returning its digest and length. Adapted from
// server/images.go GetSHA256Digest (without the log.Fatal).
func GetSHA256Digest(r io.Reader) (string, int64, error) {
	h := sha256.New()
	n, err := io.Copy(h, r)
	if err != nil {
		return "", 0, err
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil)), n, nil
}

var (
	ErrModelNotFound  = errors.New("model not found")
	ErrModelExists    = errors.New("model already exists")
	ErrBlobNotFound   = errors.New("blob not found")
	ErrBlobCorrupt    = errors.New("blob digest mismatch")
	ErrInvalidFormat  = errors.New("unsupported model format")
	ErrConvertRunning = errors.New("conversion job not found")
)

// BlobStore abstracts object storage (S3/MinIO in production, filesystem
// offline). Blobs are content-addressed by sha256 digest.
type BlobStore interface {
	// Put stores r under its computed digest, returning digest and size.
	Put(r io.Reader) (digest string, size int64, err error)
	// Get opens the blob for digest.
	Get(digest string) (io.ReadCloser, error)
	// Has reports whether digest exists.
	Has(digest string) bool
	// Delete removes digest if present.
	Delete(digest string) error
	// Stat returns blob size without opening it.
	Stat(digest string) (int64, error)
}

// DetectFormat sniffs the model format from the stream magic bytes,
// adapted from fs/gguf/reader.go usage: GGUF starts with "GGUF".
// Anything else that parses as a zip/safetensors header is labeled
// safetensors; unknown input is an error.
func DetectFormat(r io.Reader) (format string, err error) {
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidFormat, err)
	}
	switch string(magic[:]) {
	case "GGUF":
		return "gguf", nil
	default:
		// safetensors files begin with an 8-byte little-endian header length
		// whose first content byte is '{'.
		if magic[0] != 0 && magic[1] == 0 {
			return "safetensors", nil
		}
		return "", ErrInvalidFormat
	}
}

// Registry maps model names to manifests and owns the blob store.
type Registry struct {
	mu        sync.RWMutex
	manifests map[string]*Manifest
	Blobs     BlobStore
}

// NewRegistry returns a registry backed by store.
func NewRegistry(store BlobStore) *Registry {
	return &Registry{manifests: make(map[string]*Manifest), Blobs: store}
}

// Register stores a model blob and manifest under name, adapted from
// server/create.go's createModel layer assembly.
func (r *Registry) Register(name string, blob io.Reader, labels map[string]string) (*ModelInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.manifests[name]; ok {
		return nil, ErrModelExists
	}

	// Peek at the first 4 bytes to detect the format, then store the blob.
	head, rest, err := peek(blob, 4)
	if err != nil {
		return nil, err
	}
	format, err := DetectFormat(head)
	if err != nil {
		return nil, err
	}
	digest, size, err := r.Blobs.Put(rest)
	if err != nil {
		return nil, err
	}

	m := &Manifest{
		Name: name,
		Config: Layer{
			MediaType: "application/vnd.ollama.image.config",
			Digest:    digest,
			Size:      size,
		},
		Layers: []Layer{{
			MediaType: "application/vnd.ollama.image.model",
			Digest:    digest,
			Size:      size,
		}},
		Labels:  labels,
		Created: time.Now(),
	}
	r.manifests[name] = m
	return r.infoLocked(name, m, format, size), nil
}

func (r *Registry) infoLocked(name string, m *Manifest, format string, size int64) *ModelInfo {
	return &ModelInfo{
		Name:           name,
		Digest:         m.Digest(),
		SizeBytes:      size,
		Format:         format,
		Labels:         m.Labels,
		ModifiedAtUnix: m.Created.UnixMilli(),
	}
}

// peek reads the first n bytes and returns them along with a reader that
// replays the full stream.
func peek(r io.Reader, n int) (io.Reader, io.Reader, error) {
	buf := make([]byte, n)
	read, err := io.ReadFull(r, buf)
	if err != nil && read == 0 {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidFormat, err)
	}
	buf = buf[:read]
	return bytesReader(buf), io.MultiReader(bytesReader(buf), r), nil
}

func bytesReader(b []byte) io.Reader {
	return &sliceReader{b: b}
}

type sliceReader struct {
	b []byte
	i int
}

func (s *sliceReader) Read(p []byte) (int, error) {
	if s.i >= len(s.b) {
		return 0, io.EOF
	}
	n := copy(p, s.b[s.i:])
	s.i += n
	return n, nil
}

// Get returns the manifest and info for name.
func (r *Registry) Get(name string) (*Manifest, *ModelInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.manifests[name]
	if !ok {
		return nil, nil, ErrModelNotFound
	}
	var size int64
	format := "gguf"
	if len(m.Layers) > 0 {
		size = m.Layers[0].Size
		if f, ok := m.Labels["format"]; ok {
			format = f
		}
	}
	return m, r.infoLocked(name, m, format, size), nil
}

// List returns all registered models sorted by name (ListModels RPC).
func (r *Registry) List() []*ModelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*ModelInfo, 0, len(r.manifests))
	for name, m := range r.manifests {
		format := "gguf"
		if f, ok := m.Labels["format"]; ok {
			format = f
		}
		var size int64
		if len(m.Layers) > 0 {
			size = m.Layers[0].Size
		}
		out = append(out, r.infoLocked(name, m, format, size))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Delete removes a model manifest and prunes unreferenced blobs, adapted
// from server/images.go deleteUnusedLayers.
func (r *Registry) Delete(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.manifests[name]
	if !ok {
		return false
	}
	delete(r.manifests, name)

	// Reference count remaining manifests before deleting blobs.
	used := make(map[string]int)
	for _, other := range r.manifests {
		used[other.Config.Digest]++
		for _, l := range other.Layers {
			used[l.Digest]++
		}
	}
	for _, l := range append(m.Layers, m.Config) {
		if used[l.Digest] == 0 {
			_ = r.Blobs.Delete(l.Digest)
		}
	}
	return true
}

// ensure interface conformance in tests/docs.
var _ fs.FS
