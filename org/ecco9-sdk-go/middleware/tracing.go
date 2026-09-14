// Package middleware provides the shared gRPC interceptors used by every
// ecco9 cognitive service: distributed tracing context propagation, auth
// token propagation, and circuit breaking. Interceptors are transport-neutral
// where possible so the same behavior applies to unary and streaming calls.
package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"google.golang.org/grpc/metadata"
)

// Metadata keys propagated between services on every call. The trace keys
// follow the W3C Trace Context header names so the service mesh (Istio)
// and the observability stack (Tempo/Jaeger) can join spans without
// translation.
const (
	// TraceparentHeader carries the W3C trace context for a call.
	TraceparentHeader = "traceparent"
	// TracestateHeader carries vendor-specific trace extensions.
	TracestateHeader = "tracestate"
	// AuthorizationHeader carries the caller's bearer token.
	AuthorizationHeader = "authorization"
	// IdentityHeader names the Deep Tree Echo identity the call acts on.
	IdentityHeader = "x-ecco9-identity"
)

// TraceContext is the propagated portion of W3C Trace Context. It is a
// value object: invalid inputs are dropped rather than partially applied.
type TraceContext struct {
	TraceID string // 32 lowercase hex chars
	SpanID  string // 16 lowercase hex chars
	Flags   string // 2 lowercase hex chars, e.g. "01" for sampled
}

// Valid reports whether the context can be serialized as a traceparent.
func (tc TraceContext) Valid() bool {
	return len(tc.TraceID) == 32 && len(tc.SpanID) == 16 && len(tc.Flags) == 2
}

// Traceparent renders the W3C traceparent header value.
func (tc TraceContext) Traceparent() string {
	return "00-" + tc.TraceID + "-" + tc.SpanID + "-" + tc.Flags
}

// ParseTraceparent parses a W3C traceparent header of the form
// "00-<32 hex>-<16 hex>-<2 hex>". It returns false for malformed input.
func ParseTraceparent(s string) (TraceContext, bool) {
	if len(s) != 55 || s[2] != '-' || s[35] != '-' || s[52] != '-' {
		return TraceContext{}, false
	}
	tc := TraceContext{TraceID: s[3:35], SpanID: s[36:52], Flags: s[53:55]}
	if !isLowerHex(tc.TraceID) || !isLowerHex(tc.SpanID) || !isLowerHex(tc.Flags) {
		return TraceContext{}, false
	}
	return tc, true
}

// NewTraceContext generates a fresh root trace context, marked sampled.
func NewTraceContext() TraceContext {
	return TraceContext{
		TraceID: randHex(16),
		SpanID:  randHex(8),
		Flags:   "01",
	}
}

// Child returns a new trace context that shares the trace ID with tc but
// has a fresh span ID, or a fresh root context when tc is invalid.
func (tc TraceContext) Child() TraceContext {
	if !tc.Valid() {
		return NewTraceContext()
	}
	return TraceContext{TraceID: tc.TraceID, SpanID: randHex(8), Flags: tc.Flags}
}

// InjectTraceContext writes the traceparent into outgoing gRPC metadata.
func InjectTraceContext(md metadata.MD, tc TraceContext) metadata.MD {
	if !tc.Valid() {
		return md
	}
	md = md.Copy()
	md.Set(TraceparentHeader, tc.Traceparent())
	return md
}

// ExtractTraceContext reads the traceparent from incoming gRPC metadata.
// It returns false when the header is absent or malformed.
func ExtractTraceContext(md metadata.MD) (TraceContext, bool) {
	vals := md.Get(TraceparentHeader)
	if len(vals) == 0 {
		return TraceContext{}, false
	}
	return ParseTraceparent(vals[0])
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failures are effectively impossible on supported
		// platforms; fall back to a zero ID rather than panicking in a
		// serving path.
		return hex.EncodeToString(make([]byte, n))
	}
	return hex.EncodeToString(b)
}

func isLowerHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return len(s) > 0
}
