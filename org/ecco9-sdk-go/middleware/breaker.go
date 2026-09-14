package middleware

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// BreakerState is the circuit breaker lifecycle state.
type BreakerState int

const (
	// StateClosed passes all requests through.
	StateClosed BreakerState = iota
	// StateOpen rejects requests until the reset timeout elapses.
	StateOpen
	// StateHalfOpen admits a single probe request.
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

// ErrCircuitOpen is returned by client interceptors when the breaker is
// open and the call was not attempted.
var ErrCircuitOpen = status.Error(codes.Unavailable, "ecco9: circuit breaker open")

// CircuitBreaker implements the classic three-state breaker pattern shared
// by every ecco9 client: after FailureThreshold consecutive failures the
// circuit opens for ResetTimeout; the next request then probes in half-open
// state and either closes the circuit on success or re-opens on failure.
//
// The zero value is unusable; construct with NewCircuitBreaker.
type CircuitBreaker struct {
	mu               sync.Mutex
	state            BreakerState
	consecutiveFails int
	openedAt         time.Time

	FailureThreshold int
	ResetTimeout     time.Duration
}

// NewCircuitBreaker builds a breaker with the platform defaults used by
// cognitive service clients.
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
		b.tripLocked()
		return
	}
	b.consecutiveFails++
	if b.consecutiveFails >= b.FailureThreshold {
		b.tripLocked()
	}
}

func (b *CircuitBreaker) tripLocked() {
	b.state = StateOpen
	b.openedAt = time.Now()
}

// State returns the current breaker state.
func (b *CircuitBreaker) State() BreakerState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// UnaryClientBreaker returns a unary client interceptor that guards calls
// with the breaker. Only transport-level and transient failures
// (Unavailable, DeadlineExceeded, Internal) count toward tripping; caller
// errors such as InvalidArgument do not.
func UnaryClientBreaker(b *CircuitBreaker) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if !b.Allow() {
			return ErrCircuitOpen
		}
		err := invoker(ctx, method, req, reply, cc, opts...)
		recordBreakerResult(b, err)
		return err
	}
}

// StreamClientBreaker returns a stream client interceptor that guards
// stream establishment with the breaker.
func StreamClientBreaker(b *CircuitBreaker) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		if !b.Allow() {
			return nil, ErrCircuitOpen
		}
		stream, err := streamer(ctx, desc, cc, method, opts...)
		recordBreakerResult(b, err)
		return stream, err
	}
}

// recordBreakerResult folds an RPC outcome into the breaker state.
func recordBreakerResult(b *CircuitBreaker, err error) {
	if err == nil {
		b.RecordSuccess()
		return
	}
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded, codes.Internal:
		b.RecordFailure()
	default:
		// Application-level rejections (InvalidArgument, PermissionDenied,
		// NotFound, ...) indicate a healthy callee and reset the breaker.
		b.RecordSuccess()
	}
}
