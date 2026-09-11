// Package runner implements the ecco9-runner control plane: model registry,
// KV cache structures, load/unload lifecycle, and an inference job queue with
// a pluggable backend. Adapted from runner/ollamarunner/runner.go,
// runner/llamarunner/runner.go, kvcache/cache.go, llm/server.go, and
// llm/memory.go. The ggml/llama.cpp cgo backend is not portable here, so the
// backend is an interface with an offline stub.
package runner

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ServerStatus mirrors llm.ServerStatus for external health reporting.
type ServerStatus int

const (
	StatusLoadingModel ServerStatus = iota
	StatusReady
	StatusError
	StatusUnloaded
)

func (s ServerStatus) String() string {
	switch s {
	case StatusLoadingModel:
		return "loading"
	case StatusReady:
		return "ready"
	case StatusError:
		return "error"
	case StatusUnloaded:
		return "unloaded"
	}
	return "unknown"
}

// LoadParams mirrors the LoadModelRequest proto fields plus runner flags.
type LoadParams struct {
	ModelPath   string
	GPULayers   int32
	ContextSize int32
}

// LoadedModel is one registered model instance.
type LoadedModel struct {
	ID          string
	Path        string
	GPULayers   int32
	ContextSize int32
	Status      ServerStatus
	Progress    float32
	LoadedAt    time.Time
	Cache       *KVCache
}

// MemoryEstimate approximates llm.MemoryEstimate: how much VRAM a load will
// consume given gpu layers and context size.
type MemoryEstimate struct {
	Layers    int
	VRAMBytes uint64
	Fully     bool // all layers on GPU
}

// EstimateMemory is a simplified adaptation of llm.EstimateGPULayers: model
// weights plus KV cache for the requested context window.
func EstimateMemory(sizeBytes uint64, totalLayers int, p LoadParams) MemoryEstimate {
	layers := int(p.GPULayers)
	if layers <= 0 || layers > totalLayers {
		layers = totalLayers
	}
	// Per-layer weight share plus KV: 2 (K+V) * ctx * 4096 dims * 2 bytes.
	kv := uint64(p.ContextSize) * 2 * 4096 * 2
	vram := sizeBytes*uint64(layers)/uint64(max(totalLayers, 1)) + kv
	return MemoryEstimate{Layers: layers, VRAMBytes: vram, Fully: layers >= totalLayers}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var (
	ErrModelNotFound = errors.New("model not loaded")
	ErrModelExists   = errors.New("model already loaded")
	ErrQueueFull     = errors.New("inference queue full")
)

// ChatMessage mirrors the proto ChatMessage.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// GenerateRequest mirrors the proto GenerateRequest.
type GenerateRequest struct {
	RequestID   string        `json:"request_id"`
	ModelID     string        `json:"model_id"`
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int32         `json:"max_tokens"`
}

// GenerateResponse mirrors the proto GenerateResponse.
type GenerateResponse struct {
	RequestID        string `json:"request_id"`
	Content          string `json:"content"`
	PromptTokens     int32  `json:"prompt_tokens"`
	CompletionTokens int32  `json:"completion_tokens"`
	Done             bool   `json:"done"`
}

// Job is one queued inference request awaiting a sequence slot.
type Job struct {
	Req     *GenerateRequest
	Results chan GenerateResponse
	Err     chan error
}

// Backend is the pluggable inference engine (ggml/llama.cpp in production,
// stub offline). Load prepares weights; Forward consumes a prompt and
// streams generated tokens through the emit callback.
type Backend interface {
	Load(params LoadParams) error
	Forward(modelID string, req *GenerateRequest, emit func(token string, done bool)) (promptTokens, completionTokens int, err error)
	Unload() error
}

// Registry tracks loaded models by ID, mirroring llm/server.go's model
// scheduling table.
type Registry struct {
	mu     sync.RWMutex
	models map[string]*LoadedModel
	nextID int
}

// NewRegistry returns an empty model registry.
func NewRegistry() *Registry {
	return &Registry{models: make(map[string]*LoadedModel)}
}

// Load registers a model and prepares its KV cache, transitioning
// loading -> ready. Adapted from Server.loadModel in the llamarunner.
func (r *Registry) Load(p LoadParams, b Backend) (*LoadedModel, error) {
	r.mu.Lock()
	for _, m := range r.models {
		if m.Path == p.ModelPath && m.Status != StatusUnloaded {
			r.mu.Unlock()
			return nil, fmt.Errorf("%w: %s", ErrModelExists, p.ModelPath)
		}
	}
	r.nextID++
	m := &LoadedModel{
		ID:          fmt.Sprintf("model-%d", r.nextID),
		Path:        p.ModelPath,
		GPULayers:   p.GPULayers,
		ContextSize: p.ContextSize,
		Status:      StatusLoadingModel,
	}
	r.models[m.ID] = m
	r.mu.Unlock()

	if err := b.Load(p); err != nil {
		r.mu.Lock()
		m.Status = StatusError
		r.mu.Unlock()
		return nil, fmt.Errorf("load %s: %w", p.ModelPath, err)
	}

	r.mu.Lock()
	m.Progress = 1.0
	m.Status = StatusReady
	m.LoadedAt = time.Now()
	m.Cache = NewKVCache(int(p.ContextSize), 8) // capacity=ctx, 8 parallel seqs
	r.mu.Unlock()
	return m, nil
}

// Unload removes a model and frees its KV cache.
func (r *Registry) Unload(modelID string, b Backend) bool {
	r.mu.Lock()
	m, ok := r.models[modelID]
	if ok {
		delete(r.models, modelID)
		m.Status = StatusUnloaded
		if m.Cache != nil {
			m.Cache.Close()
		}
	}
	r.mu.Unlock()
	if !ok {
		return false
	}
	_ = b.Unload()
	return true
}

// Get returns a loaded model by ID or path.
func (r *Registry) Get(idOrPath string) (*LoadedModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, m := range r.models {
		if m.ID == idOrPath || m.Path == idOrPath {
			return m, nil
		}
	}
	return nil, ErrModelNotFound
}

// List returns all loaded models.
func (r *Registry) List() []*LoadedModel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*LoadedModel, 0, len(r.models))
	for _, m := range r.models {
		out = append(out, m)
	}
	return out
}
