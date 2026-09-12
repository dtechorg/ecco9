package runner

import (
	"context"
	"fmt"
	"strings"
)

// StubBackend is the offline inference Backend. It stands in for the
// ggml/llama.cpp engine: Load validates parameters and Forward emits
// deterministic tokens so the control plane (queue, KV cache, streaming)
// can be exercised without GPU/cgo.
type StubBackend struct {
	loaded *LoadParams
}

// Load records the parameters a real backend would mmap/offload.
func (s *StubBackend) Load(p LoadParams) error {
	if p.ModelPath == "" {
		return fmt.Errorf("model path required")
	}
	cp := p
	s.loaded = &cp
	return nil
}

// Forward synthesizes tokens from the prompt, emitting one per callback.
func (s *StubBackend) Forward(modelID string, req *GenerateRequest, emit func(token string, done bool)) (int, int, error) {
	if s.loaded == nil {
		return 0, 0, fmt.Errorf("no model loaded in stub backend")
	}
	var prompt strings.Builder
	for _, m := range req.Messages {
		prompt.WriteString(m.Content)
		prompt.WriteByte(' ')
	}
	promptTokens := len(strings.Fields(prompt.String()))
	text := fmt.Sprintf("[stub %s] %s", modelID, strings.TrimSpace(prompt.String()))
	tokens := strings.Fields(text)
	if req.MaxTokens > 0 && len(tokens) > int(req.MaxTokens) {
		tokens = tokens[:req.MaxTokens]
	}
	for i, tok := range tokens {
		emit(tok, i == len(tokens)-1)
	}
	return promptTokens, len(tokens), nil
}

// Unload releases the stub's reference.
func (s *StubBackend) Unload() error {
	s.loaded = nil
	return nil
}

// Queue is the inference job queue, adapted from the ollamarunner Server
// loop: sequences wait on seqsSem for a parallel slot, then are processed
// in FIFO batches. Here a bounded channel plus worker goroutines play the
// role of seqsSem + nextSeq scheduling.
type Queue struct {
	jobs    chan *Job
	backend Backend
	reg     *Registry
}

// NewQueue creates a queue with the given parallel worker count and depth.
func NewQueue(parallel, depth int, b Backend, reg *Registry) *Queue {
	if parallel <= 0 {
		parallel = 1
	}
	if depth <= 0 {
		depth = 64
	}
	q := &Queue{jobs: make(chan *Job, depth), backend: b, reg: reg}
	for i := 0; i < parallel; i++ {
		go q.worker()
	}
	return q
}

// Submit enqueues a job or returns ErrQueueFull without blocking.
func (q *Queue) Submit(ctx context.Context, job *Job) error {
	select {
	case q.jobs <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return ErrQueueFull
	}
}

func (q *Queue) worker() {
	for job := range q.jobs {
		q.process(job)
	}
}

// process runs one job: resolve model, acquire a KV slot, stream tokens.
func (q *Queue) process(job *Job) {
	// Both channels must reach a terminal state on every return path. The HTTP
	// adapter drains them until both are closed; leaving Results open after an
	// early error would otherwise block the request forever.
	defer close(job.Results)
	defer close(job.Err)

	req := job.Req
	id := req.ModelID
	if id == "" {
		id = req.Model
	}
	m, err := q.reg.Get(id)
	if err != nil {
		job.Err <- err
		return
	}
	// First model wins when the request doesn't pin one (gateway parity:
	// Generate against the default loaded model).
	if m == nil {
		list := q.reg.List()
		if len(list) == 0 {
			job.Err <- ErrModelNotFound
			return
		}
		m = list[0]
	}

	slot, err := m.Cache.Acquire(-1)
	if err != nil {
		job.Err <- err
		return
	}
	defer func() {
		m.Cache.Release(slot.SeqID)
		// Remove the sequence's transient context, keeping the cache warm
		// only within a single request (numKeep == 0).
		_ = m.Cache.Remove(slot.SeqID, 0, math_MaxInt32)
	}()

	var held *GenerateResponse
	emit := func(token string, done bool) {
		resp := GenerateResponse{
			RequestID: req.RequestID,
			Content:   token,
			Done:      done,
		}
		if done {
			held = &resp // hold back until Forward returns token counts
			return
		}
		job.Results <- resp
	}
	promptTokens, completionTokens, err := q.backend.Forward(m.ID, req, emit)
	if err != nil {
		job.Err <- err
		return
	}
	if held == nil {
		held = &GenerateResponse{RequestID: req.RequestID, Done: true}
	}
	held.PromptTokens = int32(promptTokens)
	held.CompletionTokens = int32(completionTokens)
	job.Results <- *held
	_ = m.Cache.Put(slot.SeqID, make([]int32, promptTokens+completionTokens))
}

const math_MaxInt32 = 1<<31 - 1
