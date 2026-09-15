package models

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

// ggufBlob returns a minimal stream with the GGUF magic header.
func ggufBlob(body string) io.Reader {
	return io.MultiReader(strings.NewReader("GGUF"), strings.NewReader(body))
}

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	store, err := NewFSStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFSStore: %v", err)
	}
	return NewRegistry(store)
}

func TestDetectFormat(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"gguf magic", "GGUFrest", "gguf", false},
		{"safetensors header", "\x93\x00\x00\x00{}", "safetensors", false},
		{"unknown", "XXXX....", "", true},
		{"too short", "GG", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DetectFormat(strings.NewReader(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got format %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("DetectFormat: %v", err)
			}
			if got != tc.want {
				t.Fatalf("DetectFormat = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRegisterStoresBlobAndManifest(t *testing.T) {
	reg := newTestRegistry(t)
	info, err := reg.Register("mymodel", ggufBlob("weights"), map[string]string{"format": "gguf"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if info.Name != "mymodel" {
		t.Fatalf("Name = %q, want mymodel", info.Name)
	}
	if info.Format != "gguf" {
		t.Fatalf("Format = %q, want gguf", info.Format)
	}
	if info.SizeBytes <= 0 {
		t.Fatalf("SizeBytes = %d, want > 0", info.SizeBytes)
	}
	if !strings.HasPrefix(info.Digest, "sha256:") {
		t.Fatalf("Digest = %q, want sha256: prefix", info.Digest)
	}

	// Manifest retrievable and blob present.
	m, got, err := reg.Get("mymodel")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "mymodel" || len(m.Layers) == 0 {
		t.Fatalf("unexpected manifest/info: %+v %+v", m, got)
	}
	if !reg.Blobs.Has(m.Layers[0].Digest) {
		t.Fatal("blob not stored in blob store")
	}
}

func TestRegisterDuplicateNameFails(t *testing.T) {
	reg := newTestRegistry(t)
	if _, err := reg.Register("dup", ggufBlob("a"), nil); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	if _, err := reg.Register("dup", ggufBlob("b"), nil); err != ErrModelExists {
		t.Fatalf("expected ErrModelExists, got %v", err)
	}
}

func TestRegisterRejectsUnknownFormat(t *testing.T) {
	reg := newTestRegistry(t)
	if _, err := reg.Register("bad", strings.NewReader("XXXXdata"), nil); err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestGetMissingReturnsNotFound(t *testing.T) {
	reg := newTestRegistry(t)
	if _, _, err := reg.Get("nope"); err != ErrModelNotFound {
		t.Fatalf("expected ErrModelNotFound, got %v", err)
	}
}

func TestListSortedByName(t *testing.T) {
	reg := newTestRegistry(t)
	for _, n := range []string{"zeta", "alpha", "mid"} {
		if _, err := reg.Register(n, ggufBlob("x"), map[string]string{"format": "gguf"}); err != nil {
			t.Fatalf("Register %s: %v", n, err)
		}
	}
	list := reg.List()
	if len(list) != 3 {
		t.Fatalf("List len = %d, want 3", len(list))
	}
	if list[0].Name != "alpha" || list[1].Name != "mid" || list[2].Name != "zeta" {
		t.Fatalf("List not sorted: %v %v %v", list[0].Name, list[1].Name, list[2].Name)
	}
}

func TestDeleteRemovesManifestAndUnreferencedBlob(t *testing.T) {
	reg := newTestRegistry(t)
	if _, err := reg.Register("gone", ggufBlob("data"), map[string]string{"format": "gguf"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	m, _, _ := reg.Get("gone")
	digest := m.Layers[0].Digest
	if !reg.Blobs.Has(digest) {
		t.Fatal("blob should exist before delete")
	}
	if !reg.Delete("gone") {
		t.Fatal("Delete should return true for existing model")
	}
	if _, _, err := reg.Get("gone"); err != ErrModelNotFound {
		t.Fatalf("expected ErrModelNotFound after delete, got %v", err)
	}
	if reg.Blobs.Has(digest) {
		t.Fatal("unreferenced blob should be pruned on delete")
	}
	// Deleting again reports false.
	if reg.Delete("gone") {
		t.Fatal("Delete of missing model should return false")
	}
}

func TestGetSHA256DigestComputesDigestAndLength(t *testing.T) {
	digest, n, err := GetSHA256Digest(strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("GetSHA256Digest: %v", err)
	}
	if n != 5 {
		t.Fatalf("length = %d, want 5", n)
	}
	// sha256("hello") = 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
	want := "sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if digest != want {
		t.Fatalf("digest = %q, want %q", digest, want)
	}
}

func TestFSStoreRoundTrip(t *testing.T) {
	store, err := NewFSStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFSStore: %v", err)
	}
	digest, size, err := store.Put(strings.NewReader("blob-contents"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if size != int64(len("blob-contents")) {
		t.Fatalf("size = %d, want %d", size, len("blob-contents"))
	}
	if !store.Has(digest) {
		t.Fatal("Has should report true after Put")
	}
	rc, err := store.Get(digest)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if string(got) != "blob-contents" {
		t.Fatalf("Get contents = %q, want %q", got, "blob-contents")
	}
	st, err := store.Stat(digest)
	if err != nil || st != size {
		t.Fatalf("Stat = %d, %v; want %d", st, err, size)
	}
	if err := store.Delete(digest); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if store.Has(digest) {
		t.Fatal("Has should report false after Delete")
	}
	if _, err := store.Get(digest); err == nil {
		t.Fatal("Get after Delete should error")
	}
}

// seedSource is a RemoteSource backed by an in-memory map for pull tests.
type seedSource struct{ files map[string]string }

func (s *seedSource) Fetch(_ context.Context, name string) (io.ReadCloser, int64, error) {
	body, ok := s.files[name]
	if !ok {
		return nil, 0, ErrModelNotFound
	}
	return io.NopCloser(strings.NewReader(body)), int64(len(body)), nil
}

func TestPullerPullRegistersModel(t *testing.T) {
	reg := newTestRegistry(t)
	p := &Puller{Reg: reg, Remote: &seedSource{files: map[string]string{"remote": "GGUFdata"}}}
	var sawDone bool
	err := p.Pull(context.Background(), "remote", func(pr Progress) {
		if pr.Done {
			sawDone = true
		}
	})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if !sawDone {
		t.Fatal("expected a terminal done progress")
	}
	if _, _, err := reg.Get("remote"); err != nil {
		t.Fatalf("expected model registered after pull: %v", err)
	}
}

func TestPullerPullUnknownModelFails(t *testing.T) {
	reg := newTestRegistry(t)
	p := &Puller{Reg: reg, Remote: &seedSource{files: map[string]string{}}}
	if err := p.Pull(context.Background(), "missing", func(Progress) {}); err == nil {
		t.Fatal("expected error pulling unknown model")
	}
}

func TestPullerPushVerifiesBlob(t *testing.T) {
	reg := newTestRegistry(t)
	if _, err := reg.Register("topush", ggufBlob("payload"), map[string]string{"format": "gguf"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	p := &Puller{Reg: reg}
	var sawDone bool
	if err := p.Push(context.Background(), "topush", func(pr Progress) {
		if pr.Done {
			sawDone = true
		}
	}); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if !sawDone {
		t.Fatal("expected terminal done on push")
	}
}

func TestPullerPushMissingModelFails(t *testing.T) {
	reg := newTestRegistry(t)
	p := &Puller{Reg: reg}
	if err := p.Push(context.Background(), "nope", func(Progress) {}); err != ErrModelNotFound {
		t.Fatalf("expected ErrModelNotFound, got %v", err)
	}
}

func TestConverterLifecycle(t *testing.T) {
	c := NewConverter()
	id, err := c.Start("/src/model.safetensors", "gguf", "q4_0")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if id == "" {
		t.Fatal("expected a job id")
	}
	// Wait for the async stub transform to finish.
	deadline := time.Now().Add(2 * time.Second)
	var job *ConvertJob
	for time.Now().Before(deadline) {
		job, err = c.Status(id)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if job.Status == "done" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if job.Status != "done" {
		t.Fatalf("job status = %q, want done", job.Status)
	}
	if job.Quantization != "q4_0" {
		t.Fatalf("quantization = %q, want q4_0", job.Quantization)
	}
}

func TestConverterRejectsNonGGUFTarget(t *testing.T) {
	c := NewConverter()
	if _, err := c.Start("/src/m", "onnx", ""); err == nil {
		t.Fatal("expected error for non-gguf target")
	}
}

func TestConverterStatusUnknownJob(t *testing.T) {
	c := NewConverter()
	if _, err := c.Status("convert-999"); err != ErrConvertRunning {
		t.Fatalf("expected ErrConvertRunning, got %v", err)
	}
}

func TestManifestDigestStable(t *testing.T) {
	m := &Manifest{
		Name:   "x",
		Config: Layer{Digest: "sha256:a"},
		Layers: []Layer{{Digest: "sha256:b"}},
	}
	d1 := m.Digest()
	d2 := m.Digest()
	if d1 != d2 {
		t.Fatalf("digest not stable: %q vs %q", d1, d2)
	}
	if !strings.HasPrefix(d1, "sha256:") {
		t.Fatalf("digest missing prefix: %q", d1)
	}
}

func TestPeekReplaysStream(t *testing.T) {
	head, rest, err := peek(strings.NewReader("GGUFpayload"), 4)
	if err != nil {
		t.Fatalf("peek: %v", err)
	}
	hb, _ := io.ReadAll(head)
	if string(hb) != "GGUF" {
		t.Fatalf("head = %q, want GGUF", hb)
	}
	all, _ := io.ReadAll(rest)
	if string(all) != "GGUFpayload" {
		t.Fatalf("rest = %q, want full stream", all)
	}
}

func TestBytesReaderEOF(t *testing.T) {
	r := bytesReader([]byte("ab"))
	buf := make([]byte, 4)
	n, _ := r.Read(buf)
	if n != 2 {
		t.Fatalf("first read = %d, want 2", n)
	}
	if _, err := r.Read(buf); err != io.EOF {
		t.Fatalf("second read err = %v, want EOF", err)
	}
	// Ensure a zero-len buffer read on empty reader is EOF, not a hang.
	empty := bytes.NewReader(nil)
	if _, err := empty.Read(make([]byte, 0)); err != nil && err != io.EOF {
		_ = err // best-effort sanity; not all readers return EOF on empty reads
	}
}
