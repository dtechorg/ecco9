package gateway

import (
	"sync"
	"time"
)

// BreakerState is the circuit breaker lifecycle state.
type BreakerState int

const (
	StateClosed BreakerState = iota
	StateOpen
	StateHalfOpen
)

func (s BreakerState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	}
	return "unknown"
}

// CircuitBreaker implements the classic three-state breaker pattern:
// after FailureThreshold consecutive failures the circuit opens for
// ResetTimeout; the next request then probes in half-open state and either
// closes the circuit on success or re-opens on failure.
type CircuitBreaker struct {
	mu               sync.Mutex
	state            BreakerState
	consecutiveFails int
	FailureThreshold int
	ResetTimeout     time.Duration
	openedAt         time.Time
}

// NewCircuitBreaker builds a breaker with sensible gateway defaults.
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{FailureThreshold: 5, ResetTimeout: 30 * time.Second}
}

// Allow reports whether a request may pass through the breaker.
func (b *CircuitBreaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == StateOpen {
		if time.Since(b.openedAt) >= b.ResetTimeout {
			b.state = StateHalfOpen // allow one probe
			return true
		}
		return false
	}
	return true
}

// RecordSuccess closes the circuit and resets the failure counter.
func (b *CircuitBreaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.consecutiveFails = 0
	b.state = StateClosed
}

// RecordFailure counts a failure, opening the circuit at the threshold.
func (b *CircuitBreaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == StateHalfOpen {
		b.trip()
		return
	}
	b.consecutiveFails++
	if b.consecutiveFails >= b.FailureThreshold {
		b.trip()
	}
}

func (b *CircuitBreaker) trip() {
	b.state = StateOpen
	b.openedAt = time.Now()
}

// State returns the current breaker state.
func (b *CircuitBreaker) State() BreakerState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// RateLimiter is a token-bucket rate limiter. Capacity equals the per-minute
// allowance; tokens refill continuously.
type RateLimiter struct {
	mu       sync.Mutex
	rate     float64 // tokens per second
	capacity float64
	tokens   float64
	last     time.Time
}

// NewRateLimiter creates a limiter; requestsPerMinute <= 0 means unlimited.
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	if requestsPerMinute <= 0 {
		requestsPerMinute = 1 << 30
	}
	return &RateLimiter{
		rate:     float64(requestsPerMinute) / 60,
		capacity: float64(requestsPerMinute),
		tokens:   float64(requestsPerMinute),
		last:     time.Now(),
	}
}

// Allow consumes one token if available.
func (l *RateLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.tokens += now.Sub(l.last).Seconds() * l.rate
	if l.tokens > l.capacity {
		l.tokens = l.capacity
	}
	l.last = now
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}

// ProviderStats tracks per-provider usage, latency, tokens, and cost.
type ProviderStats struct {
	mu               sync.Mutex
	Requests         int64
	Successes        int64
	Failures         int64
	TotalLatency     time.Duration
	PromptTokens     int64
	CompletionTokens int64
	TotalCostUSD     float64
	LastUsed         time.Time
	LastError        string
}

// Snapshot is a consistent copy of ProviderStats.
func (s *ProviderStats) Snapshot() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	var avg float64
	if s.Requests > 0 {
		avg = float64(s.TotalLatency.Milliseconds()) / float64(s.Requests)
	}
	return map[string]any{
		"requests":          s.Requests,
		"successes":         s.Successes,
		"failures":          s.Failures,
		"avg_latency_ms":    avg,
		"prompt_tokens":     s.PromptTokens,
		"completion_tokens": s.CompletionTokens,
		"total_cost_usd":    s.TotalCostUSD,
		"last_used":         s.LastUsed,
		"last_error":        s.LastError,
	}
}
