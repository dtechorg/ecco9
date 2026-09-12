package middleware

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type authContextKey struct{}

// Validator authenticates a bearer token and returns the identity it is
// bound to. Implementations live in ecco9-identity; the SDK only defines
// the seam so every service enforces auth the same way.
type Validator interface {
	ValidateToken(ctx context.Context, token string) (identityID string, err error)
}

// ValidatorFunc adapts a plain function to Validator.
type ValidatorFunc func(ctx context.Context, token string) (string, error)

// ValidateToken implements Validator.
func (f ValidatorFunc) ValidateToken(ctx context.Context, token string) (string, error) {
	return f(ctx, token)
}

// IdentityFromContext returns the authenticated identity ID installed by
// the auth server interceptors, or "" when the call is unauthenticated.
func IdentityFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(authContextKey{}).(string); ok {
		return v
	}
	return ""
}

// UnaryServerAuth returns a unary server interceptor that authenticates the
// bearer token in the "authorization" metadata header and installs the
// resulting identity in the handler context.
func UnaryServerAuth(v Validator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx, err := authenticate(ctx, v)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// StreamServerAuth returns a stream server interceptor that authenticates
// once at stream establishment and installs the identity for the handler.
func StreamServerAuth(v Validator) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx, err := authenticate(ss.Context(), v)
		if err != nil {
			return err
		}
		return handler(srv, &contextServerStream{ServerStream: ss, ctx: ctx})
	}
}

// UnaryClientAuth returns a unary client interceptor that attaches the
// bearer token to every outgoing call.
func UnaryClientAuth(token string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(withBearer(ctx, token), method, req, reply, cc, opts...)
	}
}

// StreamClientAuth returns a stream client interceptor that attaches the
// bearer token to every outgoing stream.
func StreamClientAuth(token string) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return streamer(withBearer(ctx, token), desc, cc, method, opts...)
	}
}

// authenticate extracts and validates the bearer token from incoming
// metadata, returning a context carrying the authenticated identity.
func authenticate(ctx context.Context, v Validator) (context.Context, error) {
	if v == nil {
		return ctx, status.Error(codes.Unauthenticated, "ecco9: no auth validator configured")
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx, status.Error(codes.Unauthenticated, "ecco9: missing call metadata")
	}
	token, err := bearerFromMetadata(md)
	if err != nil {
		return ctx, err
	}
	identityID, err := v.ValidateToken(ctx, token)
	if err != nil {
		return ctx, status.Errorf(codes.Unauthenticated, "ecco9: invalid token: %v", err)
	}
	return context.WithValue(ctx, authContextKey{}, identityID), nil
}

func bearerFromMetadata(md metadata.MD) (string, error) {
	vals := md.Get(AuthorizationHeader)
	if len(vals) == 0 {
		return "", status.Error(codes.Unauthenticated, "ecco9: missing authorization token")
	}
	const prefix = "Bearer "
	s := vals[0]
	if len(s) <= len(prefix) || s[:len(prefix)] != prefix {
		return "", status.Error(codes.Unauthenticated, "ecco9: malformed authorization header")
	}
	return s[len(prefix):], nil
}

func withBearer(ctx context.Context, token string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, AuthorizationHeader, "Bearer "+token)
}

// contextServerStream wraps a ServerStream with an overridden Context.
type contextServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *contextServerStream) Context() context.Context { return s.ctx }
