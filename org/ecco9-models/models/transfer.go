package models

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"sync"
	"time"
)

// Progress mirrors api.ProgressResponse / the streaming Pull/Push RPCs.
type Progress struct {
	Status         string `json:"status"`
	Digest         string `json:"digest,omitempty"`
	CompletedBytes int64  `json:"completed_bytes"`
	TotalBytes     int64  `json:"total_bytes"`
	Done           bool   `json:"done"`
}

// newBackoff returns the monorepo's n² randomized backoff (adapted from
// server/download.go newBackoff).
func newBackoff(maxBackoff time.Duration) func(ctx context.Context) error {
	var n int
	return func(ctx context.Context) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		n++
		// n^2 backoff is a little smoother than the common 2^n choice.
		d := min(time.Duration(n*n)*10*time.Millisecond, maxBackoff)
		// Randomize between 0.5-1.5x to prevent thundering herds.
		d = time.Duration(float64(d) * (rand.Float64() + 0.5))
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			return nil
		}
	}
}

// RemoteSource abstracts the upstream model registry (the HTTP registry in
// the monorepo). The offline implementation serves from a seed directory.
type RemoteSource interface {
	// Fetch streams the named model blob and reports its total size.
	Fetch(ctx context.Context, name string) (io.ReadCloser, int64, error)
}

// Puller performs pull/push transfers against the local blob store,
// adapted from server/download.go (blobDownload parts) and upload.go
// (progressWriter), simplified to single-stream copies with the same
// progress callback shape.
type Puller struct {
	Reg    *Registry
	Remote RemoteSource
}

// Pull downloads name from the remote source into the registry, reporting
// progress. Mirrors PullModel.
func (p *Puller) Pull(ctx context.Context, name string, fn func(Progress)) error {
	if p.Remote == nil {
		return fmt.Errorf("no remote source configured")
	}
	fn(Progress{Status: "pulling manifest"})
	rc, total, err := p.Remote.Fetch(ctx, name)
	if err != nil {
		return err
	}
	defer rc.Close()

	fn(Progress{Status: "pulling blob", TotalBytes: total})
	pr := &progressReader{r: rc, total: total, fn: func(n int64) {
		fn(Progress{Status: "pulling blob", CompletedBytes: n, TotalBytes: total})
	}}

	backoff := newBackoff(10 * time.Second)
	var digest string
	var size int64
	for tries := 0; tries < 3; tries++ {
		digest, size, err = p.Reg.Blobs.Put(pr)
		if err == nil {
			break
		}
		if berr := backoff(ctx); berr != nil {
			return berr
		}
	}
	if err != nil {
		return err
	}

	// Register the manifest if the name is new (idempotent pulls skip).
	if _, _, err := p.Reg.Get(name); err == ErrModelNotFound {
		m := &Manifest{
			Name:    name,
			Config:  Layer{MediaType: "application/vnd.ollama.image.config", Digest: digest, Size: size},
			Layers:  []Layer{{MediaType: "application/vnd.ollama.image.model", Digest: digest, Size: size}},
			Labels:  map[string]string{"format": "gguf"},
			Created: time.Now(),
		}
		p.Reg.mu.Lock()
		p.Reg.manifests[name] = m
		p.Reg.mu.Unlock()
	}
	fn(Progress{Status: "success", Digest: digest, CompletedBytes: total, TotalBytes: total, Done: true})
	return nil
}

// Push uploads name's blobs to the configured sink, adapted from upload.go.
// The offline sink is a no-op writer: blobs are verified locally instead.
func (p *Puller) Push(_ context.Context, name string, fn func(Progress)) error {
	m, _, err := p.Reg.Get(name)
	if err != nil {
		return err
	}
	fn(Progress{Status: "pushing manifest"})
	for _, layer := range append(m.Layers, m.Config) {
		rc, err := p.Reg.Blobs.Get(layer.Digest)
		if err != nil {
			return err
		}
		// Verify blob integrity before "upload" (verifyBlob in images.go).
		digest, n, err := GetSHA256Digest(rc)
		rc.Close()
		if err != nil {
			return err
		}
		if digest != layer.Digest {
			return fmt.Errorf("%w: %s", ErrBlobCorrupt, layer.Digest)
		}
		fn(Progress{Status: "pushing blob", Digest: digest, CompletedBytes: n, TotalBytes: n})
	}
	fn(Progress{Status: "success", Done: true})
	return nil
}

// progressReader reports cumulative bytes, adapted from progressWriter.
type progressReader struct {
	r     io.Reader
	n     int64
	total int64
	fn    func(int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.n += int64(n)
	if p.fn != nil {
		p.fn(p.n)
	}
	return n, err
}

// ConvertJob tracks one conversion lifecycle, adapted from
// convert/convert.go's ConvertModel pipeline (safetensors -> gguf with
// optional quantization).
type ConvertJob struct {
	ID           string    `json:"id"`
	SourcePath   string    `json:"source_path"`
	TargetFormat string    `json:"target_format"`
	Quantization string    `json:"quantization"`
	Status       string    `json:"status"` // queued, converting, quantizing, done, error
	Error        string    `json:"error,omitempty"`
	Started      time.Time `json:"started"`
	Finished     time.Time `json:"finished,omitempty"`
}

// Converter manages asynchronous conversion jobs.
type Converter struct {
	mu   sync.Mutex
	jobs map[string]*ConvertJob
	next int
}

// NewConverter returns an empty converter.
func NewConverter() *Converter {
	return &Converter{jobs: make(map[string]*ConvertJob)}
}

// Start enqueues a conversion and returns its job ID (ConvertModel RPC).
// The actual tensor conversion runs asynchronously; this control-plane
// implementation walks the lifecycle states with a stub transform.
func (c *Converter) Start(sourcePath, targetFormat, quantization string) (string, error) {
	if targetFormat != "gguf" {
		return "", fmt.Errorf("%w: target %s", ErrInvalidFormat, targetFormat)
	}
	c.mu.Lock()
	c.next++
	job := &ConvertJob{
		ID:           fmt.Sprintf("convert-%d", c.next),
		SourcePath:   sourcePath,
		TargetFormat: targetFormat,
		Quantization: quantization,
		Status:       "queued",
		Started:      time.Now(),
	}
	c.jobs[job.ID] = job
	c.mu.Unlock()

	go c.run(job)
	return job.ID, nil
}

// run walks the conversion phases, mirroring convertModelFromFiles ->
// quantizeLayer progress reporting in server/create.go.
func (c *Converter) run(job *ConvertJob) {
	set := func(status, errMsg string) {
		c.mu.Lock()
		job.Status = status
		job.Error = errMsg
		if status == "done" || status == "error" {
			job.Finished = time.Now()
		}
		c.mu.Unlock()
	}
	set("converting", "")
	time.Sleep(10 * time.Millisecond) // stub tensor transform
	if job.Quantization != "" {
		set("quantizing", "")
		time.Sleep(10 * time.Millisecond) // stub quantization pass
	}
	set("done", "")
}

// Status returns a job by ID.
func (c *Converter) Status(id string) (*ConvertJob, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	j, ok := c.jobs[id]
	if !ok {
		return nil, ErrConvertRunning
	}
	cp := *j
	return &cp, nil
}

// Jobs lists all conversion jobs.
func (c *Converter) Jobs() []*ConvertJob {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*ConvertJob, 0, len(c.jobs))
	for _, j := range c.jobs {
		cp := *j
		out = append(out, &cp)
	}
	return out
}
