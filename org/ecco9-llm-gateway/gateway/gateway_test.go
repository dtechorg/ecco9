package gateway

import (
	"context"
	"strings"
	"testing"
	"time"
)

// testBackends returns three providers at distinct priorities so selection
// order is deterministic: anthropic 100 > openai 70 > local 10.
func testBackends() []*Backend {
	return []*Backend{
		{Provider: ProviderLocalGGUF, BaseURL: "http://local", Model: "local", Priority: 10},
		{Provider: ProviderOpenAI, APIKey: "k", BaseURL: "https://api.openai.com", Model: "gpt", Priority: 70,
			CostPer1KInput: 0.0004, CostPer1KOutput: 0.0016},
		{Provider: ProviderAnthropic, APIKey: "k", BaseURL: "https://api.anthropic.com", Model: "claude", Priority: 100,
			CostPer1KInput: 0.003, CostPer1KOutput: 0.015},
	}
}

func newTestGateway(t *testing.T, tr Transport) *Gateway {
	t.Helper()
	g, err := New(testBackends(), tr)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return g
}

func TestGeneratePrefersHighestPriorityProvider(t *testing.T) {
	g := newTestGateway(t, &StubTransport{})
	resp, err := g.Generate(context.Background(), &GenerateRequest{
		RequestID: "r1",
		Messages:  []ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.ProviderUsed != ProviderAnthropic {
		t.Fatalf("ProviderUsed = %v, want anthropic (highest priority)", resp.ProviderUsed)
	}
	if resp.RequestID != "r1" || !resp.Done {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if g.CurrentProvider() != "anthropic" {
		t.Fatalf("CurrentProvider = %q, want anthropic", g.CurrentProvider())
	}
}

func TestGeneratePreferredProviderFirst(t *testing.T) {
	g := newTestGateway(t, &StubTransport{})
	resp, err := g.Generate(context.Background(), &GenerateRequest{
		RequestID:         "r2",
		PreferredProvider: ProviderLocalGGUF,
		Messages:          []ChatMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.ProviderUsed != ProviderLocalGGUF {
		t.Fatalf("ProviderUsed = %v, want preferred local_gguf", resp.ProviderUsed)
	}
}

func TestGenerateFailoverFallsThroughToNextProvider(t *testing.T) {
	// Force anthropic (top priority) to fail; openai should serve.
	tr := &StubTransport{Fail: map[Provider]bool{ProviderAnthropic: true}}
	g := newTestGateway(t, tr)
	resp, err := g.Generate(context.Background(), &GenerateRequest{
		RequestID: "r3",
		Messages:  []ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.ProviderUsed != ProviderOpenAI {
		t.Fatalf("ProviderUsed = %v, want openai after anthropic failure", resp.ProviderUsed)
	}
}

func TestGenerateAllProvidersFailReturnsError(t *testing.T) {
	tr := &StubTransport{Fail: map[Provider]bool{
		ProviderAnthropic: true,
		ProviderOpenAI:    true,
		ProviderLocalGGUF: true,
	}}
	g := newTestGateway(t, tr)
	if _, err := g.Generate(context.Background(), &GenerateRequest{
		Messages: []ChatMessage{{Role: "user", Content: "x"}},
	}); err == nil {
		t.Fatal("expected error when all providers fail")
	}
}

func TestGenerateRecordsCostTracking(t *testing.T) {
	g := newTestGateway(t, &StubTransport{})
	if _, err := g.Generate(context.Background(), &GenerateRequest{
		RequestID: "cost",
		Messages:  []ChatMessage{{Role: "user", Content: "count my tokens please"}},
	}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	stats := g.Stats()
	st, ok := stats["anthropic"]
	if !ok {
		t.Fatalf("stats missing anthropic entry: %v", stats)
	}
	if st["requests"].(int64) != 1 || st["successes"].(int64) != 1 {
		t.Fatalf("expected 1 request/1 success, got %v", st)
	}
	if st["total_cost_usd"].(float64) <= 0 {
		t.Fatalf("expected positive cost, got %v", st["total_cost_usd"])
	}
	if st["prompt_tokens"].(int64) == 0 {
		t.Fatal("expected prompt tokens to be tracked")
	}
}

func TestCircuitBreakerOpensAfterThresholdAndRecovers(t *testing.T) {
	b := NewCircuitBreaker()
	b.FailureThreshold = 3
	b.ResetTimeout = 20 * time.Millisecond

	for i := 0; i < 3; i++ {
		if !b.Allow() {
			t.Fatalf("call %d should be allowed while closed", i)
		}
		b.RecordFailure()
	}
	if b.State() != StateOpen {
		t.Fatalf("state = %v, want open after threshold failures", b.State())
	}
	if b.Allow() {
		t.Fatal("open circuit should reject requests before reset timeout")
	}

	time.Sleep(25 * time.Millisecond)
	if !b.Allow() {
		t.Fatal("half-open probe should be allowed after reset timeout")
	}
	b.RecordSuccess()
	if b.State() != StateClosed {
		t.Fatalf("state = %v, want closed after successful probe", b.State())
	}
}

func TestCircuitBreakerHalfOpenFailureReopens(t *testing.T) {
	b := NewCircuitBreaker()
	b.FailureThreshold = 1
	b.ResetTimeout = 10 * time.Millisecond
	b.RecordFailure()
	if b.State() != StateOpen {
		t.Fatalf("state = %v, want open", b.State())
	}
	time.Sleep(15 * time.Millisecond)
	if !b.Allow() { // probe
		t.Fatal("expected half-open probe")
	}
	b.RecordFailure() // probe failed
	if b.State() != StateOpen {
		t.Fatalf("state = %v, want re-opened after probe failure", b.State())
	}
}

func TestRateLimiterEnforcesCapacity(t *testing.T) {
	l := NewRateLimiter(60) // capacity 60, but starts full
	// Drain the full bucket; every call within the burst should pass.
	for i := 0; i < 60; i++ {
		if !l.Allow() {
			t.Fatalf("call %d within capacity should be allowed", i)
		}
	}
	// Bucket now empty; refill is 1/sec so an immediate call must fail.
	if l.Allow() {
		t.Fatal("call beyond capacity should be rejected")
	}
}

func TestRateLimiterUnlimitedWhenZero(t *testing.T) {
	l := NewRateLimiter(0)
	for i := 0; i < 100; i++ {
		if !l.Allow() {
			t.Fatalf("unlimited limiter rejected call %d", i)
		}
	}
}

func TestProviderStatusReportsHealthAndLatency(t *testing.T) {
	tr := &StubTransport{Fail: map[Provider]bool{ProviderAnthropic: true}}
	g := newTestGateway(t, tr)
	if _, err := g.Generate(context.Background(), &GenerateRequest{
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	health, latency := g.ProviderStatus()
	if len(health) != 3 {
		t.Fatalf("expected 3 providers in health map, got %v", health)
	}
	// anthropic recorded one failure but breaker is not yet open (threshold 5).
	if !health["anthropic"] {
		t.Fatal("anthropic should still be healthy below failure threshold")
	}
	if _, ok := latency["openai"]; !ok {
		t.Fatalf("latency map missing openai: %v", latency)
	}
}

func TestStubTransportSynthesizesDeterministicCompletion(t *testing.T) {
	tr := &StubTransport{}
	content, usage, err := tr.Complete(context.Background(), "https://api.openai.com", "k", &ChatCompletion{
		Model:    "m",
		Messages: []ChatMessage{{Role: "user", Content: "one two three"}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !strings.Contains(content, "one two three") {
		t.Fatalf("completion missing prompt echo: %q", content)
	}
	if usage.PromptTokens != 3 {
		t.Fatalf("PromptTokens = %d, want 3", usage.PromptTokens)
	}
}

func TestDefaultBackendsRegistersKeyedProviders(t *testing.T) {
	env := map[string]string{
		"OPENAI_API_KEY":    "sk",
		"ANTHROPIC_API_KEY": "ak",
	}
	backends := DefaultBackends(func(k string) string { return env[k] })
	// openai + anthropic + always-on local gguf.
	if len(backends) != 3 {
		t.Fatalf("expected 3 backends, got %d: %v", len(backends), backends)
	}
	var sawLocal bool
	for _, b := range backends {
		if b.Provider == ProviderLocalGGUF {
			sawLocal = true
		}
	}
	if !sawLocal {
		t.Fatal("local gguf backend should be registered by default")
	}
}

func TestDefaultBackendsHonorsDisableLocalGGUF(t *testing.T) {
	backends := DefaultBackends(func(k string) string {
		if k == "ECCO9_DISABLE_LOCAL_GGUF" {
			return "1"
		}
		return ""
	})
	if len(backends) != 0 {
		t.Fatalf("expected no backends when local disabled and no keys, got %v", backends)
	}
}

func TestProviderStringRoundTrip(t *testing.T) {
	cases := map[Provider]string{
		ProviderOpenAI:      "openai",
		ProviderAnthropic:   "anthropic",
		ProviderOpenRouter:  "openrouter",
		ProviderFeatherless: "featherless",
		ProviderLocalGGUF:   "local_gguf",
		ProviderUnspecified: "unspecified",
	}
	for p, want := range cases {
		if got := p.String(); got != want {
			t.Errorf("Provider(%d).String() = %q, want %q", int(p), got, want)
		}
	}
}
