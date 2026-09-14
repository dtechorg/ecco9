package gateway

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// StubTransport is the offline Transport implementation. It synthesizes a
// deterministic completion locally so the gateway compiles and runs without
// network access; production deployments inject an HTTP transport instead.
type StubTransport struct {
	// Fail, when set, forces every call to error (used to exercise failover).
	Fail map[Provider]bool
}

// Complete returns a synthesized echo completion with rough token counts.
func (s *StubTransport) Complete(_ context.Context, baseURL, _ string, req *ChatCompletion) (string, Usage, error) {
	if s.Fail != nil && s.Fail[providerFromURL(baseURL)] {
		return "", Usage{}, fmt.Errorf("%w: stub forced failure for %s", ErrUnavailable, baseURL)
	}
	var prompt strings.Builder
	for _, m := range req.Messages {
		prompt.WriteString(m.Content)
		prompt.WriteByte(' ')
	}
	text := strings.TrimSpace(prompt.String())
	content := fmt.Sprintf("[stub %s] %s", req.Model, text)
	if req.MaxTokens > 0 && len(strings.Fields(content)) > int(req.MaxTokens) {
		content = strings.Join(strings.Fields(content)[:req.MaxTokens], " ")
	}
	usage := Usage{
		PromptTokens:     len(strings.Fields(text)),
		CompletionTokens: len(strings.Fields(content)),
	}
	return content, usage, nil
}

func providerFromURL(u string) Provider {
	switch {
	case strings.Contains(u, "anthropic"):
		return ProviderAnthropic
	case strings.Contains(u, "openrouter"):
		return ProviderOpenRouter
	case strings.Contains(u, "openai"):
		return ProviderOpenAI
	case strings.Contains(u, "featherless"):
		return ProviderFeatherless
	default:
		return ProviderLocalGGUF
	}
}

// Gateway is the multi-provider router: registry, selection, failover.
// Adapted from MultiProviderLLM in core/deeptreeecho/multi_provider_llm.go.
type Gateway struct {
	mu        sync.RWMutex
	providers []*registeredProvider
	current   int
	stats     map[string]*ProviderStats
}

// New builds a gateway from backends, wiring each with a breaker+limiter.
func New(backends []*Backend, t Transport) (*Gateway, error) {
	if len(backends) == 0 {
		return nil, errNoBackends
	}
	g := &Gateway{stats: make(map[string]*ProviderStats)}
	for _, b := range backends {
		g.Register(b, t)
	}
	return g, nil
}

// Register adds a backend to the registry.
func (g *Gateway) Register(b *Backend, t Transport) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.providers = append(g.providers, &registeredProvider{
		backend:   b,
		transport: t,
		breaker:   NewCircuitBreaker(),
		limiter:   NewRateLimiter(b.RequestsPerMinute),
	})
	g.stats[b.Provider.String()] = &ProviderStats{}
}

// Generate routes a request through provider selection with failover,
// mirroring GenerateThought's try-current-then-fallback loop.
func (g *Gateway) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	g.mu.RLock()
	if len(g.providers) == 0 {
		g.mu.RUnlock()
		return nil, errNoBackends
	}
	order := providerOrder(g.providers, req.PreferredProvider)
	g.mu.RUnlock()

	var lastErr error
	for _, rp := range order {
		if !rp.available() {
			continue
		}
		st := g.stats[rp.backend.Provider.String()]
		content, usage, err := rp.complete(ctx, req, st)
		if err != nil {
			lastErr = err
			continue
		}
		// Track the provider that served this request by registry index so
		// CurrentProvider reflects the last success regardless of sort order.
		if idx := indexOf(g.providers, rp); idx != g.current {
			g.mu.Lock()
			g.current = idx
			g.mu.Unlock()
		}
		return &GenerateResponse{
			RequestID:        req.RequestID,
			Content:          content,
			ProviderUsed:     rp.backend.Provider,
			PromptTokens:     int32(usage.PromptTokens),
			CompletionTokens: int32(usage.CompletionTokens),
			Done:             true,
		}, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("all LLM providers failed or unavailable")
	}
	return nil, lastErr
}

func indexOf(all []*registeredProvider, rp *registeredProvider) int {
	for i, p := range all {
		if p == rp {
			return i
		}
	}
	return 0
}

// ProviderStatus mirrors GetProviderStatus: health and average latency maps.
func (g *Gateway) ProviderStatus() (health map[string]bool, latency map[string]float64) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	health = make(map[string]bool, len(g.providers))
	latency = make(map[string]float64, len(g.providers))
	for _, rp := range g.providers {
		name := rp.backend.Provider.String()
		health[name] = rp.breaker.State() != StateOpen
		if st, ok := g.stats[name]; ok {
			snap := st.Snapshot()
			latency[name], _ = snap["avg_latency_ms"].(float64)
		}
	}
	return health, latency
}

// Stats returns per-provider statistics snapshots.
func (g *Gateway) Stats() map[string]map[string]any {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make(map[string]map[string]any, len(g.stats))
	for name, st := range g.stats {
		snap := st.Snapshot()
		for _, rp := range g.providers {
			if rp.backend.Provider.String() == name {
				snap["circuit"] = rp.breaker.State().String()
			}
		}
		out[name] = snap
	}
	return out
}

// CurrentProvider names the provider that served the last success.
func (g *Gateway) CurrentProvider() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.current < len(g.providers) {
		return g.providers[g.current].backend.Provider.String()
	}
	return "none"
}
