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

// AnonSub / AnonProsoponId identify the seeded anon prosopon (001_init.sql) used
// by the dev-anon fallback when auth is disabled.
const (
	AnonSub        = "anon"
	AnonProsoponId = int64(2)
)

// ContextWithSub stores only the sub — used for the dev-mode anon path where
// no prosopon ID is available.
func ContextWithSub(ctx context.Context, sub string) context.Context {
	return context.WithValue(ctx, subKey, sub)
}

// contextWithAnon stores the anon identity (sub + prosopon ID) for the dev-anon
// fallback, so both Chat (needs sub) and conversation resolution (needs prosopon
// ID) work without a token. No conversation ID — handlers resolve it per prosopon.
func contextWithAnon(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, subKey, AnonSub)
	ctx = context.WithValue(ctx, prosoponIdKey, AnonProsoponId)
	return ctx
}

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
// Returns (0, false) in dev-mode (anon) or when the token predates this field.
func ProsoponIdFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(prosoponIdKey).(int64)
	return id, ok && id != 0
}

// ConversationIDFromContext retrieves the conversation ID set by the interceptor.
// Returns (0, false) in dev-mode (anon) or when the token predates this field.
func ConversationIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(convIdKey).(int64)
	return id, ok && id != 0
}

// AuthInterceptor verifies the server-issued JWT on every RPC except those
// under /celine.v1.Hermes/ (the auth flow itself doesn't need a token).
//
// If verifier is nil (no CELINE_JWT_SECRET set), auth is skipped and every
// request is treated as "anon" — useful for local dev without Google OAuth.
type AuthInterceptor struct {
	verifier *Verifier
	devAnon  bool
}

func NewAuthInterceptor(v *Verifier) *AuthInterceptor {
	return &AuthInterceptor{verifier: v}
}

// NewDevAnonInterceptor builds an interceptor that bypasses token verification
// entirely and treats every request as the anon prosopon. Local dev only —
// gated behind CELINE_DEV_ANON at the call site. Never wire this in prod.
func NewDevAnonInterceptor() *AuthInterceptor {
	return &AuthInterceptor{devAnon: true}
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
	if strings.HasPrefix(procedure, "/celine.v1.Hermes/") {
		return ctx, nil
	}

	// Dev-anon fallback: auth disabled, every request is the anon prosopon.
	if a.devAnon {
		return contextWithAnon(ctx), nil
	}

	// Dev mode: no JWT secret configured — pass through as "anon".
	if a.verifier == nil {
		return ContextWithSub(ctx, "anon"), nil
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
