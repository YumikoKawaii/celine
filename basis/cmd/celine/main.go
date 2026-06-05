package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/YumikoKawaii/celine/basis/gen/celine/v1/celinev1connect"
	"github.com/YumikoKawaii/celine/basis/internal/agent"
	"github.com/YumikoKawaii/celine/basis/internal/hermes"
	"github.com/YumikoKawaii/celine/basis/internal/llm"
	"github.com/YumikoKawaii/celine/basis/internal/mneme"
	"github.com/YumikoKawaii/celine/basis/internal/rpc"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	addr := os.Getenv("CELINE_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	db, err := mneme.NewPool(ctx, mustEnv("CELINE_DB_DSN"))
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	rdb := mneme.NewRedis(mustEnv("CELINE_REDIS_ADDR"))
	defer rdb.Close()

	// Auth — Verifier is nil when CELINE_JWT_SECRET is unset; interceptor
	// falls back to "anon" so local dev works without Google OAuth.
	var verifier *hermes.Verifier
	if secret := os.Getenv("CELINE_JWT_SECRET"); secret != "" {
		verifier = hermes.NewVerifier(secret)
	}
	interceptor := hermes.NewAuthInterceptor(verifier)
	opts := connect.WithInterceptors(interceptor)

	// Celine service
	brain := llm.New(mustEnv("ANTHROPIC_API_KEY"), os.Getenv("CELINE_MODEL"))
	convs := mneme.NewConversationStore(db)
	msgs := mneme.NewMessageStore(db, rdb)
	celineSvc := rpc.NewCelineService(agent.New(brain, agent.SystemPrompt(), convs, msgs))

	// Hermes service
	var googleAuth *hermes.GoogleAuth
	var issuer *hermes.Issuer
	if clientID := os.Getenv("GOOGLE_CLIENT_ID"); clientID != "" {
		googleAuth = hermes.NewGoogleAuth(clientID, mustEnv("GOOGLE_CLIENT_SECRET"))
		issuer = hermes.NewIssuer(mustEnv("CELINE_JWT_SECRET"))
	}
	hermesSvc := rpc.NewHermesService(googleAuth, issuer, mneme.NewClientStore(db))

	mux := http.NewServeMux()
	celinePath, celineHandler := celinev1connect.NewCelineHandler(celineSvc, opts)
	hermesPath, hermesHandler := celinev1connect.NewHermesHandler(hermesSvc, opts)
	mux.Handle(celinePath, celineHandler)
	mux.Handle(hermesPath, hermesHandler)

	srv := &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(devCORS(mux), &http2.Server{}),
	}

	log.Printf("celine backend listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s is required", key)
	}
	return v
}

func devCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers",
			"Content-Type, Connect-Protocol-Version, Connect-Timeout-Ms, X-User-Agent, X-Grpc-Web, Authorization")
		w.Header().Set("Access-Control-Expose-Headers",
			"Grpc-Status, Grpc-Message, Grpc-Status-Details-Bin")
		w.Header().Set("Access-Control-Max-Age", "7200")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}
