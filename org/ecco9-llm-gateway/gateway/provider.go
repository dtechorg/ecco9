// Package gateway implements the unified multi-provider LLM gateway core:
// provider registry, circuit breakers, failover, rate limiting, and cost
// tracking. Adapted from core/deeptreeecho/multi_provider_llm.go,
// openai_provider.go, anthropic_provider.go, openrouter_provider.go,
// featherless_client.go, and llm_client.go.
package gateway

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Provider enumerates the supported LLM backends (mirrors proto enum).
type Provider int

const (
	ProviderUnspecified Provider = iota
	ProviderOpenAI
	ProviderAnthropic
	ProviderOpenRouter
	ProviderFeatherless
	ProviderLocalGGUF
)

func (p Provider) String() string {
	switch p {
	case ProviderOpenAI:
		return "openai"
	case ProviderAnthropic:
		return "anthropic"
	case ProviderOpenRouter:
		return "openrouter"
	case ProviderFeatherless:
		return "featherless"
	case ProviderLocalGGUF:
		return "local_gguf"
	}
	return "unspecified"
}

// ChatMessage is a single conversational turn (mirrors proto ChatMessage).
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// GenerateRequest mirrors the proto GenerateRequest.
type GenerateRequest struct {
	RequestID         string        `json:"request_id"`
	Model             string        `json:"model"`
	Messages          []ChatMessage `json:"messages"`
	PreferredProvider Provider      `json:"preferred_provider"`
	Temperature       float64       `json:"temperature"`
	MaxTokens         int32         `json:"max_tokens"`
	Stream            bool          `json:"stream"`
}

// GenerateResponse mirrors the proto GenerateResponse.
type GenerateResponse struct {
	RequestID        string   `json:"request_id"`
	Content          string   `json:"content"`
	ProviderUsed     Provider `json:"provider_used"`
	PromptTokens     int32    `json:"prompt_tokens"`
	CompletionTokens int32    `json:"completion_tokens"`
	Done             bool     `json:"done"`
}

// ChatCompletion is the wire-level request a provider transport sends.
type ChatCompletion struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int32         `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
}

// Usage reports token accounting for cost tracking.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// Transport abstracts the HTTP call to a provider backend so the gateway
// compiles and runs offline. Real deployments inject an HTTP transport;
// tests and offline runs use the stub in stub.go.
type Transport interface {
	// Complete issues one chat-completion call against the provider endpoint
	// identified by baseURL, authenticated with apiKey.
	Complete(ctx context.Context, baseURL, apiKey string, req *ChatCompletion) (content string, usage Usage, err error)
}

// ErrUnavailable marks a provider as temporarily unusable.
var ErrUnavailable = errors.New("provider not available")

// Backend is one configured LLM provider endpoint.
type Backend struct {
	Provider Provider
	APIKey   string
	BaseURL  string
	Model    string
	// Priority: higher is preferred (anthropic 100, openrouter 90, openai 70,
	// per the monorepo defaults).
	Priority int
	// CostPer1KInput/CostPer1KOutput drive cost tracking, USD per 1K tokens.
	CostPer1KInput  float64
	CostPer1KOutput float64
	// RequestsPerMinute bounds the token-bucket rate limiter (0 = unlimited).
	RequestsPerMinute int
}

// endpoint returns the chat-completions URL for the backend.
func (b *Backend) endpoint() string {
	switch b.Provider {
	case ProviderAnthropic:
		return b.BaseURL + "/v1/messages"
	default: // OpenAI-compatible: openai, openrouter, featherless, local gguf
		return b.BaseURL + "/v1/chat/completions"
	}
}

// defaultBackends mirrors initializeProviders() in the monorepo: providers
// are only registered when their API key is present. Local GGUF needs no key.
func DefaultBackends(getenv func(string) string) []*Backend {
	var out []*Backend
	if k := getenv("ANTHROPIC_API_KEY"); k != "" {
		out = append(out, &Backend{Provider: ProviderAnthropic, APIKey: k,
			BaseURL: "https://api.anthropic.com", Model: "claude-3-5-sonnet-20241022",
			Priority: 100, CostPer1KInput: 0.003, CostPer1KOutput: 0.015, RequestsPerMinute: 60})
	}
	if k := getenv("OPENROUTER_API_KEY"); k != "" {
		out = append(out, &Backend{Provider: ProviderOpenRouter, APIKey: k,
			BaseURL: "https://openrouter.ai/api", Model: "anthropic/claude-3.5-sonnet",
			Priority: 90, CostPer1KInput: 0.003, CostPer1KOutput: 0.015, RequestsPerMinute: 120})
	}
	if k := getenv("OPENAI_API_KEY"); k != "" {
		out = append(out, &Backend{Provider: ProviderOpenAI, APIKey: k,
			BaseURL: "https://api.openai.com", Model: "gpt-4.1-mini",
			Priority: 70, CostPer1KInput: 0.0004, CostPer1KOutput: 0.0016, RequestsPerMinute: 500})
	}
	if k := getenv("FEATHERLESS_API_KEY"); k != "" {
		out = append(out, &Backend{Provider: ProviderFeatherless, APIKey: k,
			BaseURL: "https://api.featherless.ai", Model: "default",
			Priority: 50, CostPer1KInput: 0.0002, CostPer1KOutput: 0.0002, RequestsPerMinute: 60})
	}
	if getenv("ECCO9_DISABLE_LOCAL_GGUF") == "" {
		out = append(out, &Backend{Provider: ProviderLocalGGUF,
			BaseURL: getenv("ECCO9_RUNNER_URL"), Model: "local",
			Priority: 10, CostPer1KInput: 0, CostPer1KOutput: 0, RequestsPerMinute: 0})
	}
	return out
}

var errNoBackends = fmt.Errorf("no LLM providers configured")

// registeredProvider couples a backend with its transport and breakers.
type registeredProvider struct {
	backend   *Backend
	transport Transport
	breaker   *CircuitBreaker
	limiter   *RateLimiter
}

func (r *registeredProvider) available() bool {
	return r.breaker.Allow() && r.limiter.Allow()
}

// complete issues a request through breaker + limiter, recording stats.
func (r *registeredProvider) complete(ctx context.Context, req *GenerateRequest, st *ProviderStats) (string, Usage, error) {
	model := req.Model
	if model == "" {
		model = r.backend.Model
	}
	cc := &ChatCompletion{
		Model:       model,
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}
	start := time.Now()
	content, usage, err := r.transport.Complete(ctx, r.backend.endpoint(), r.backend.APIKey, cc)
	latency := time.Since(start)

	st.mu.Lock()
	st.Requests++
	st.TotalLatency += latency
	st.LastUsed = time.Now()
	if err != nil {
		st.Failures++
		st.LastError = err.Error()
	} else {
		st.Successes++
		st.PromptTokens += int64(usage.PromptTokens)
		st.CompletionTokens += int64(usage.CompletionTokens)
		st.TotalCostUSD += costOf(r.backend, usage)
	}
	st.mu.Unlock()

	if err != nil {
		r.breaker.RecordFailure()
	} else {
		r.breaker.RecordSuccess()
	}
	return content, usage, err
}

func costOf(b *Backend, u Usage) float64 {
	return float64(u.PromptTokens)/1000*b.CostPer1KInput +
		float64(u.CompletionTokens)/1000*b.CostPer1KOutput
}

// providerOrder returns candidates for a request: preferred provider first,
// then all others sorted by priority (bubble sort as in the monorepo).
func providerOrder(all []*registeredProvider, preferred Provider) []*registeredProvider {
	sorted := make([]*registeredProvider, len(all))
	copy(sorted, all)
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j].backend.Priority < sorted[j+1].backend.Priority {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}
	if preferred == ProviderUnspecified {
		return sorted
	}
	out := make([]*registeredProvider, 0, len(sorted))
	for _, p := range sorted {
		if p.backend.Provider == preferred {
			out = append(out, p)
		}
	}
	for _, p := range sorted {
		if p.backend.Provider != preferred {
			out = append(out, p)
		}
	}
	return out
}
