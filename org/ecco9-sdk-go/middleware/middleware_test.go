package middleware

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestTraceparentRoundTrip(t *testing.T) {
	tc := NewTraceContext()
	parsed, ok := ParseTraceparent(tc.Traceparent())
	if !ok {
		t.Fatalf("failed to parse own traceparent %q", tc.Traceparent())
	}
	if parsed != tc {
		t.Fatalf("round trip mismatch: got %+v want %+v", parsed, tc)
	}
}

func TestParseTraceparentRejectsMalformed(t *testing.T) {
	cases := []string{
		"",
		"00-abc-def-01",
		"00-ABCDEF0123456789ABCDEF0123456789-0123456789ABCDEF-01", // uppercase hex
		"01-0123456789abcdef0123456789abcdef-0123456789abcdef-01", // wrong version is fine but length check
	}
	for _, c := range cases {
		if _, ok := ParseTraceparent(c); ok && len(c) != 55 {
			t.Fatalf("accepted malformed traceparent %q", c)
		}
	}
	// Wrong-version but well-formed header still parses structurally.
	if _, ok := ParseTraceparent("01-0123456789abcdef0123456789abcdef-0123456789abcdef-01"); !ok {
		t.Fatal("well-formed version-01 traceparent should parse")
	}
}

func TestTraceContextChild(t *testing.T) {
	parent := NewTraceContext()
	child := parent.Child()
	if child.TraceID != parent.TraceID {
		t.Fatal("child must share the parent trace ID")
	}
	if child.SpanID == parent.SpanID {
		t.Fatal("child must have a fresh span ID")
	}
	// Invalid parents yield a fresh root context.
	if root := (TraceContext{}).Child(); !root.Valid() {
		t.Fatal("child of invalid context should be a valid fresh root")
	}
}

func TestExtractInjectTraceContext(t *testing.T) {
	tc := NewTraceContext()
	md := InjectTraceContext(metadata.MD{}, tc)
	got, ok := ExtractTraceContext(md)
	if !ok {
		t.Fatal("expected to extract injected trace context")
	}
	if got != tc {
		t.Fatalf("got %+v want %+v", got, tc)
	}
	// Invalid contexts must not be injected.
	if md := InjectTraceContext(metadata.MD{}, TraceContext{}); len(md.Get(TraceparentHeader)) != 0 {
		t.Fatal("invalid trace context must not be injected")
	}
}

func TestUnaryServerAuth(t *testing.T) {
	v := ValidatorFunc(func(_ context.Context, token string) (string, error) {
		if token != "good-token" {
			return "", errors.New("bad token")
		}
		return "identity-42", nil
	})
	interceptor := UnaryServerAuth(v)

	handler := func(ctx context.Context, req any) (any, error) {
		return IdentityFromContext(ctx), nil
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(AuthorizationHeader, "Bearer"+" good-token"))
	resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "identity-42" {
		t.Fatalf("got identity %v", resp)
	}

	cases := []context.Context{
		context.Background(), // no metadata
		metadata.NewIncomingContext(context.Background(), metadata.MD{}),                                                // no header
		metadata.NewIncomingContext(context.Background(), metadata.Pairs(AuthorizationHeader, "bad")),                   // malformed
		metadata.NewIncomingContext(context.Background(), metadata.Pairs(AuthorizationHeader, "Bearer"+" wrong-token")), // invalid token
	}
	for i, c := range cases {
		if _, err := interceptor(c, nil, &grpc.UnaryServerInfo{}, handler); status.Code(err) != codes.Unauthenticated {
			t.Fatalf("case %d: got %v, want Unauthenticated", i, err)
		}
	}
}

func TestUnaryClientAuth(t *testing.T) {
	interceptor := UnaryClientAuth("tok")
	var gotMD metadata.MD
	invoker := func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		gotMD, _ = metadata.FromOutgoingContext(ctx)
		return nil
	}
	if err := interceptor(context.Background(), "/m", nil, nil, nil, invoker); err != nil {
		t.Fatal(err)
	}
	vals := gotMD.Get(AuthorizationHeader)
	if len(vals) != 1 || vals[0] != "Bearer"+" tok" {
		t.Fatalf("outgoing auth header = %v", vals)
	}
}

func TestCircuitBreakerLifecycle(t *testing.T) {
	b := NewCircuitBreaker()
	b.FailureThreshold = 3
	b.ResetTimeout = 50 * time.Millisecond

	for i := 0; i < 3; i++ {
		if !b.Allow() {
			t.Fatalf("attempt %d should be allowed while closed", i)
		}
		b.RecordFailure()
	}
	if b.State() != StateOpen {
		t.Fatalf("state = %v, want open", b.State())
	}
	if b.Allow() {
		t.Fatal("open breaker must reject requests")
	}

	time.Sleep(60 * time.Millisecond)
	if !b.Allow() {
		t.Fatal("breaker should admit a half-open probe after reset timeout")
	}
	if b.State() != StateHalfOpen {
		t.Fatalf("state = %v, want half-open", b.State())
	}
	b.RecordSuccess()
	if b.State() != StateClosed {
		t.Fatalf("state = %v, want closed after probe success", b.State())
	}
}

func TestCircuitBreakerHalfOpenFailureReopens(t *testing.T) {
	b := NewCircuitBreaker()
	b.FailureThreshold = 1
	b.ResetTimeout = time.Millisecond
	b.RecordFailure()
	time.Sleep(5 * time.Millisecond)
	if !b.Allow() {
		t.Fatal("expected half-open probe")
	}
	b.RecordFailure()
	if b.State() != StateOpen {
		t.Fatalf("state = %v, want open after failed probe", b.State())
	}
}

func TestUnaryClientBreaker(t *testing.T) {
	b := NewCircuitBreaker()
	b.FailureThreshold = 2
	interceptor := UnaryClientBreaker(b)

	fail := func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		return status.Error(codes.Unavailable, "backend down")
	}
	reject := func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		return status.Error(codes.InvalidArgument, "bad request")
	}

	// Caller errors must not trip the breaker.
	for i := 0; i < 5; i++ {
		_ = interceptor(context.Background(), "/m", nil, nil, nil, reject)
	}
	if b.State() != StateClosed {
		t.Fatalf("caller errors tripped the breaker: %v", b.State())
	}

	// Transient failures trip it.
	_ = interceptor(context.Background(), "/m", nil, nil, nil, fail)
	_ = interceptor(context.Background(), "/m", nil, nil, nil, fail)
	if b.State() != StateOpen {
		t.Fatalf("state = %v, want open", b.State())
	}

	// While open, calls fail fast without invoking.
	invoked := false
	err := interceptor(context.Background(), "/m", nil, nil, nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			invoked = true
			return nil
		})
	if invoked {
		t.Fatal("open breaker must fail fast")
	}
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("got %v, want Unavailable", err)
	}
}

func TestTracingInterceptors(t *testing.T) {
	// Server: incoming traceparent is continued with a child span.
	serverInterceptor := UnaryServerTracing()
	var serverTC TraceContext
	handler := func(ctx context.Context, req any) (any, error) {
		var ok bool
		serverTC, ok = TraceContextFromContext(ctx)
		if !ok {
			t.Error("handler context missing trace context")
		}
		return nil, nil
	}
	incoming := NewTraceContext()
	ctx := metadata.NewIncomingContext(context.Background(),
		metadata.Pairs(TraceparentHeader, incoming.Traceparent()))
	if _, err := serverInterceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler); err != nil {
		t.Fatal(err)
	}
	if serverTC.TraceID != incoming.TraceID || serverTC.SpanID == incoming.SpanID {
		t.Fatalf("server trace context = %+v, want child of %+v", serverTC, incoming)
	}

	// Server without incoming trace starts a fresh trace.
	if _, err := serverInterceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler); err != nil {
		t.Fatal(err)
	}
	if !serverTC.Valid() {
		t.Fatal("server must start a fresh trace when none is propagated")
	}

	// Client: outgoing metadata carries a child of the context's trace.
	clientInterceptor := UnaryClientTracing()
	var outMD metadata.MD
	invoker := func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		outMD, _ = metadata.FromOutgoingContext(ctx)
		return nil
	}
	if err := clientInterceptor(ContextWithTraceContext(context.Background(), incoming), "/m", nil, nil, nil, invoker); err != nil {
		t.Fatal(err)
	}
	gotTC, ok := ExtractTraceContext(outMD)
	if !ok {
		t.Fatal("client did not inject traceparent")
	}
	if gotTC.TraceID != incoming.TraceID || gotTC.SpanID == incoming.SpanID {
		t.Fatalf("client trace = %+v, want child of %+v", gotTC, incoming)
	}
}
