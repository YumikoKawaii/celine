package hermes

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"connectrpc.com/connect"
)

type contextKey string

const (
	subKey        contextKey = "sub"
	prosoponIdKey contextKey = "prosopon_id"
	convIdKey     contextKey = "conv_id"
)

// contextWithClaims stores sub, prosopon ID, and conversation ID from a verified token.
func contextWithClaims(ctx context.Context, c Claims) context.Context {
	ctx = context.WithValue(ctx, subKey, c.Subject)
	ctx = context.WithValue(ctx, prosoponIdKey, c.ProsoponId)
	ctx = context.WithValue(ctx, convIdKey, c.ConversationId)
	return ctx
}

// SubFromContext retrieves the authenticated sub set by the interceptor.
func SubFromContext(ctx context.Context) (string, bool) {
	sub, ok := ctx.Value(subKey).(string)
	return sub, ok && sub != ""
}

// ProsoponIdFromContext retrieves the prosopon ID set by the interceptor.
// Returns (0, false) when the token predates this field.
func ProsoponIdFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(prosoponIdKey).(int64)
	return id, ok && id != 0
}

// ConversationIDFromContext retrieves the conversation ID set by the interceptor.
// Returns (0, false) when the token predates this field.
func ConversationIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(convIdKey).(int64)
	return id, ok && id != 0
}

// AuthInterceptor verifies the server-issued JWT on every RPC except those
// under /celine.v1.Hermes/ (the auth flow itself doesn't need a token).
// A valid token is always required — there is no unauthenticated path.
type AuthInterceptor struct {
	verifier *Verifier
}

func NewAuthInterceptor(v *Verifier) *AuthInterceptor {
	return &AuthInterceptor{verifier: v}
}

func (a *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := a.authenticate(ctx, req.Spec().Procedure, req.Header())
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (a *AuthInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (a *AuthInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := a.authenticate(ctx, conn.Spec().Procedure, conn.RequestHeader())
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

func (a *AuthInterceptor) authenticate(ctx context.Context, procedure string, headers http.Header) (context.Context, error) {
	// Hermes auth-flow routes don't require a token — they're how a token is
	// obtained. Gnorizo is the exception: it validates an existing token and
	// hydrates the user, so it must go through the normal auth check.
	if strings.HasPrefix(procedure, "/celine.v1.Hermes/") &&
		procedure != "/celine.v1.Hermes/Gnorizo" {
		return ctx, nil
	}

	raw := headers.Get("Authorization")
	token := strings.TrimPrefix(raw, "Bearer ")
	if token == "" || token == raw {
		return ctx, connect.NewError(connect.CodeUnauthenticated, errors.New("missing bearer token"))
	}

	claims, err := a.verifier.Verify(token)
	if err != nil {
		return ctx, connect.NewError(connect.CodeUnauthenticated, err)
	}
	return contextWithClaims(ctx, claims), nil
}
