package middleware

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// UnaryServerTracing returns a unary server interceptor that extracts the
// incoming trace context, derives a child span context for the handler, and
// makes it available via TraceContextFromContext. Calls arriving without a
// traceparent start a fresh trace so no cognitive step goes untraced.
func UnaryServerTracing() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(serverTraceContext(ctx), req)
	}
}

// StreamServerTracing returns a stream server interceptor that performs the
// same trace extraction once at stream establishment.
func StreamServerTracing() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, &contextServerStream{ServerStream: ss, ctx: serverTraceContext(ss.Context())})
	}
}

// UnaryClientTracing returns a unary client interceptor that continues the
// current trace (from TraceContextFromContext) or starts a new one,
// injecting the child context as the outgoing traceparent.
func UnaryClientTracing() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(clientTraceContext(ctx), method, req, reply, cc, opts...)
	}
}

// StreamClientTracing returns a stream client interceptor that injects the
// traceparent into the outgoing stream's metadata.
func StreamClientTracing() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return streamer(clientTraceContext(ctx), desc, cc, method, opts...)
	}
}

type traceContextKey struct{}

// TraceContextFromContext returns the trace context installed by the
// tracing interceptors, or false when none is present.
func TraceContextFromContext(ctx context.Context) (TraceContext, bool) {
	tc, ok := ctx.Value(traceContextKey{}).(TraceContext)
	return tc, ok
}

// ContextWithTraceContext installs tc in ctx; outgoing client calls made
// with the returned context continue this trace.
func ContextWithTraceContext(ctx context.Context, tc TraceContext) context.Context {
	return context.WithValue(ctx, traceContextKey{}, tc)
}

// serverTraceContext derives the handler trace context from incoming
// metadata, starting a fresh trace when none was propagated.
func serverTraceContext(ctx context.Context) context.Context {
	tc, ok := ExtractTraceContext(incomingMD(ctx))
	if ok {
		tc = tc.Child()
	} else {
		tc = NewTraceContext()
	}
	return context.WithValue(ctx, traceContextKey{}, tc)
}

// clientTraceContext injects the outgoing traceparent derived from the
// context's trace context, starting a fresh trace when absent.
func clientTraceContext(ctx context.Context) context.Context {
	tc, ok := TraceContextFromContext(ctx)
	if ok {
		tc = tc.Child()
	} else {
		tc = NewTraceContext()
	}
	ctx = context.WithValue(ctx, traceContextKey{}, tc)
	md, _ := metadata.FromOutgoingContext(ctx)
	return metadata.NewOutgoingContext(ctx, InjectTraceContext(md, tc))
}

func incomingMD(ctx context.Context) metadata.MD {
	md, _ := metadata.FromIncomingContext(ctx)
	return md
}
