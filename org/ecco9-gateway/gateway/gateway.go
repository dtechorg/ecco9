// Package gateway implements the external-facing ecco9 API gateway. It
// reverse-proxies Ollama-compatible, OpenAI-compatible, and Deep Tree Echo
// routes to the backend cognitive services via a route table resolved from
// environment variables with in-cluster DNS defaults.
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dtechorg/ecco9-sdk-go/contracts"
	"github.com/dtechorg/ecco9-sdk-go/health"
)

// Version is reported by /api/version when no models backend overrides it.
const Version = "0.1.0-echo"

// Service names in the ecco9-core backend topology.
const (
	SvcReservoir     = "reservoir"
	SvcMemory        = "memory"
	SvcEmotion       = "emotion"
	SvcIdentity      = "identity"
	SvcEchobeats     = "echobeats"
	SvcConsciousness = "consciousness"
	SvcAtomspace     = "atomspace"
	SvcRelevance     = "relevance"
	SvcWisdom        = "wisdom"
	SvcMetacog       = "metacog"
	SvcOntogenesis   = "ontogenesis"
	SvcEchodream     = "echodream"
	SvcLLMGateway    = "llm-gateway"
	SvcRunner        = "runner"
	SvcModels        = "models"
	SvcOrchestrator  = "orchestrator"
)

// Route describes one external path prefix and the backend service that
// terminates it.
type Route struct {
	Path    string
	Service string
}

// Routes is the canonical route table: external API surface -> cognitive
// backend service.
var Routes = []Route{
	// Ollama-compatible surface.
	{"/api/generate", SvcLLMGateway},
	{"/api/chat", SvcLLMGateway},
	{"/api/embed", SvcLLMGateway},
	{"/api/tags", SvcModels},
	{"/api/version", SvcModels},
	// OpenAI-compatible surface.
	{"/v1/chat/completions", SvcLLMGateway},
	{"/v1/completions", SvcLLMGateway},
	{"/v1/embeddings", SvcLLMGateway},
	{"/v1/models", SvcModels},
	// Deep Tree Echo surface.
	{"/api/echo/status", SvcIdentity}, // served locally by the aggregator
	{"/api/echo/think", SvcReservoir},
	{"/api/echo/feel", SvcEmotion},
	{"/api/echo/remember", SvcMemory},
}

// serviceOrder is the deterministic fan-out order for the aggregate
// /api/echo/status handler.
var serviceOrder = []string{
	SvcReservoir, SvcMemory, SvcEmotion, SvcIdentity, SvcEchobeats,
	SvcConsciousness, SvcAtomspace, SvcRelevance, SvcWisdom, SvcMetacog,
	SvcOntogenesis, SvcEchodream, SvcLLMGateway, SvcRunner, SvcModels,
	SvcOrchestrator,
}

// defaultBackends are the in-cluster DNS defaults (namespace ecco9-core).
var defaultBackends = map[string]string{
	SvcReservoir:     "http://ecco9-reservoir.ecco9-core:8081",
	SvcMemory:        "http://ecco9-memory.ecco9-core:8082",
	SvcEmotion:       "http://ecco9-emotion.ecco9-core:8083",
	SvcIdentity:      "http://ecco9-identity.ecco9-core:8084",
	SvcEchobeats:     "http://ecco9-echobeats.ecco9-core:8085",
	SvcConsciousness: "http://ecco9-consciousness.ecco9-core:8086",
	SvcAtomspace:     "http://ecco9-atomspace.ecco9-core:8087",
	SvcRelevance:     "http://ecco9-relevance.ecco9-core:8088",
	SvcWisdom:        "http://ecco9-wisdom.ecco9-core:8089",
	SvcMetacog:       "http://ecco9-metacog.ecco9-core:8090",
	SvcOntogenesis:   "http://ecco9-ontogenesis.ecco9-core:8091",
	SvcEchodream:     "http://ecco9-echodream.ecco9-core:8092",
	SvcLLMGateway:    "http://ecco9-llm-gateway.ecco9-core:8093",
	SvcRunner:        "http://ecco9-runner.ecco9-core:8094",
	SvcModels:        "http://ecco9-models.ecco9-core:8095",
	SvcOrchestrator:  "http://ecco9-orchestrator.ecco9-core:8096",
}

// ResolveBackends builds the service->base-URL map, letting environment
// variables of the form ECCO9_SVC_<NAME> (dashes become underscores, e.g.
// ECCO9_SVC_LLM_GATEWAY) override the in-cluster DNS defaults.
func ResolveBackends() map[string]string {
	out := make(map[string]string, len(defaultBackends))
	for name, def := range defaultBackends {
		env := "ECCO9_SVC_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
		if v := os.Getenv(env); v != "" {
			out[name] = strings.TrimRight(v, "/")
		} else {
			out[name] = def
		}
	}
	return out
}

// tokenBucket is a simple concurrent-safe token bucket for rate limiting.
type tokenBucket struct {
	mu     sync.Mutex
	tokens float64
	max    float64
	rate   float64 // tokens per second
	last   time.Time
}

func newTokenBucket(rate, burst float64) *tokenBucket {
	return &tokenBucket{tokens: burst, max: burst, rate: rate, last: time.Now()}
}

func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.tokens += now.Sub(b.last).Seconds() * b.rate
	if b.tokens > b.max {
		b.tokens = b.max
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// rateLimiter keeps one bucket per client key (remote IP).
type rateLimiter struct {
	mu    sync.Mutex
	bps   map[string]*tokenBucket
	rate  float64
	burst float64
}

func newRateLimiter(rate, burst float64) *rateLimiter {
	return &rateLimiter{bps: make(map[string]*tokenBucket), rate: rate, burst: burst}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	b, ok := rl.bps[key]
	if !ok {
		b = newTokenBucket(rl.rate, rl.burst)
		rl.bps[key] = b
	}
	rl.mu.Unlock()
	return b.allow()
}

// Middleware wraps the mux with rate limiting.
func (rl *rateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			key = r.RemoteAddr
		}
		if !rl.allow(key) {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error": "rate limit exceeded",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// AuthConfig configures the API-key auth middleware hook.
type AuthConfig struct {
	// Enabled toggles enforcement. When false the middleware is a no-op.
	Enabled bool
	// Header is the header carrying the API key (default "X-Ecco9-Key").
	Header string
	// Keys is the set of accepted API keys.
	Keys map[string]bool
	// Exempt are path prefixes that skip auth (e.g. /healthz, /api/version).
	Exempt []string
}

// AuthFromEnv builds AuthConfig from ECCO9_GATEWAY_API_KEYS (comma-separated).
// An empty value disables auth.
func AuthFromEnv() AuthConfig {
	cfg := AuthConfig{Header: "X-Ecco9-Key", Exempt: []string{"/healthz", "/api/version"}}
	raw := os.Getenv("ECCO9_GATEWAY_API_KEYS")
	if raw == "" {
		return cfg
	}
	cfg.Enabled = true
	cfg.Keys = make(map[string]bool)
	for _, k := range strings.Split(raw, ",") {
		if k = strings.TrimSpace(k); k != "" {
			cfg.Keys[k] = true
		}
	}
	return cfg
}

// Middleware enforces the API key header when enabled.
func (a AuthConfig) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.Enabled {
			next.ServeHTTP(w, r)
			return
		}
		for _, p := range a.Exempt {
			if strings.HasPrefix(r.URL.Path, p) {
				next.ServeHTTP(w, r)
				return
			}
		}
		if !a.Keys[r.Header.Get(a.Header)] {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": "missing or invalid API key",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// serviceHealth is one backend's contribution to the aggregate status.
type serviceHealth struct {
	Service           string  `json:"service"`
	URL               string  `json:"url"`
	Reachable         bool    `json:"reachable"`
	Status            string  `json:"status"`
	IdentityCoherence float64 `json:"identity_coherence"`
	CognitiveLoad     float64 `json:"cognitive_load"`
	Error             string  `json:"error,omitempty"`
}

// Gateway is the external-facing API gateway for the ecco9 platform.
type Gateway struct {
	backends map[string]string
	proxies  map[string]*httputil.ReverseProxy
	limiter  *rateLimiter
	auth     AuthConfig
	client   *http.Client
	health   *health.Reporter
	// FanoutTimeout bounds each /healthz probe in the status aggregator.
	FanoutTimeout time.Duration
}

// New builds a Gateway with backends resolved from the environment, default
// rate limiting (100 rps, burst 200), and auth from ECCO9_GATEWAY_API_KEYS.
func New() *Gateway {
	return NewWithConfig(ResolveBackends(), AuthFromEnv(), 100, 200)
}

// NewWithConfig builds a Gateway from explicit configuration.
func NewWithConfig(backends map[string]string, auth AuthConfig, rate, burst float64) *Gateway {
	g := &Gateway{
		backends:      backends,
		proxies:       make(map[string]*httputil.ReverseProxy),
		limiter:       newRateLimiter(rate, burst),
		auth:          auth,
		client:        &http.Client{Timeout: 10 * time.Second},
		health:        health.NewReporter(),
		FanoutTimeout: 2 * time.Second,
	}
	for svc, base := range backends {
		if u, err := url.Parse(base); err == nil {
			g.proxies[svc] = newProxy(u)
		} else {
			log.Printf("ecco9-gateway: invalid backend URL for %s: %v", svc, err)
		}
	}
	return g
}

// Backends returns the resolved service->URL map (copy).
func (g *Gateway) Backends() map[string]string {
	out := make(map[string]string, len(g.backends))
	for k, v := range g.backends {
		out[k] = v
	}
	return out
}

func newProxy(target *url.URL) *httputil.ReverseProxy {
	p := httputil.NewSingleHostReverseProxy(target)
	orig := p.Director
	p.Director = func(r *http.Request) {
		orig(r)
		r.Host = target.Host
	}
	p.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":   "backend unreachable",
			"backend": target.String(),
			"detail":  err.Error(),
		})
	}
	return p
}

// Handler returns the fully wrapped gateway handler (auth + rate limiting).
func (g *Gateway) Handler() http.Handler {
	return g.auth.Middleware(g.limiter.Middleware(g.Routes()))
}

// Routes returns the bare gateway mux without middleware.
func (g *Gateway) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", g.handleHealthz)
	mux.HandleFunc("/api/echo/status", g.handleEchoStatus)

	for _, rt := range Routes {
		route := rt
		if route.Path == "/api/echo/status" {
			continue // handled locally by the aggregator
		}
		mux.HandleFunc(route.Path, func(w http.ResponseWriter, r *http.Request) {
			g.forward(w, r, route.Service)
		})
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"service":  "ecco9-gateway",
			"identity": "Deep Tree Echo embodied cognition gateway",
			"version":  Version,
		})
	})
	return mux
}

func (g *Gateway) forward(w http.ResponseWriter, r *http.Request, service string) {
	p, ok := g.proxies[service]
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error":   "no backend configured",
			"service": service,
		})
		return
	}
	p.ServeHTTP(w, r)
}

func (g *Gateway) handleHealthz(w http.ResponseWriter, r *http.Request) {
	g.health.Handler().ServeHTTP(w, r)
}

// handleEchoStatus fans out to every backend service's /healthz endpoint and
// computes an aggregate identity coherence across the gestalt.
func (g *Gateway) handleEchoStatus(w http.ResponseWriter, r *http.Request) {
	results := make([]serviceHealth, len(serviceOrder))
	var wg sync.WaitGroup
	for i, svc := range serviceOrder {
		base, ok := g.backends[svc]
		if !ok {
			continue
		}
		wg.Add(1)
		go func(i int, svc, base string) {
			defer wg.Done()
			results[i] = g.probe(r.Context(), svc, base)
		}(i, svc, base)
	}
	wg.Wait()

	var up, sum float64
	for _, h := range results {
		if h.Reachable {
			up++
			sum += h.IdentityCoherence
		}
	}
	coherence := 1.0
	if up > 0 {
		coherence = sum / up
	}
	status := contracts.ServingStatusServing
	if up == 0 {
		status = contracts.ServingStatusNotServing
	} else if up < float64(len(results)) {
		status = contracts.ServingStatusDegraded
	}
	g.health.SetStatus(status)
	g.health.SetCoherence(coherence)
	g.health.SetLoad(1.0 - up/float64(len(results)))

	writeJSON(w, http.StatusOK, map[string]any{
		"status":             servingStatusString(mustJSON(status)),
		"identity_coherence": coherence,
		"services_up":        int(up),
		"services_total":     len(results),
		"services":           results,
	})
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func (g *Gateway) probe(ctx context.Context, svc, base string) serviceHealth {
	h := serviceHealth{Service: svc, URL: base, Status: "UNREACHABLE"}
	ctx, cancel := context.WithTimeout(ctx, g.FanoutTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/healthz", nil)
	if err != nil {
		h.Error = err.Error()
		return h
	}
	resp, err := g.client.Do(req)
	if err != nil {
		h.Error = err.Error()
		return h
	}
	defer resp.Body.Close()
	var body struct {
		Status            json.RawMessage `json:"status"`
		IdentityCoherence float64         `json:"identity_coherence"`
		CognitiveLoad     float64         `json:"cognitive_load"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		h.Error = fmt.Sprintf("decode healthz: %v", err)
		return h
	}
	h.Reachable = resp.StatusCode == http.StatusOK
	h.Status = servingStatusString(body.Status)
	h.IdentityCoherence = body.IdentityCoherence
	h.CognitiveLoad = body.CognitiveLoad
	return h
}

// servingStatusString normalizes a healthz status, accepting either the
// numeric contracts.ServingStatus encoding or a plain string.
func servingStatusString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "UNKNOWN"
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		switch contracts.ServingStatus(n) {
		case contracts.ServingStatusServing:
			return "SERVING"
		case contracts.ServingStatusNotServing:
			return "NOT_SERVING"
		case contracts.ServingStatusDegraded:
			return "DEGRADED"
		}
	}
	return "UNKNOWN"
}

// RouteTable returns the external route table for documentation/debugging.
func RouteTable() []Route {
	out := make([]Route, len(Routes))
	copy(out, Routes)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
