package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/YumikoKawaii/celine/basis/gen/celine/v1/celinev1connect"
	"github.com/YumikoKawaii/celine/basis/internal/agent"
	"github.com/YumikoKawaii/celine/basis/internal/config"
	"github.com/YumikoKawaii/celine/basis/internal/ergon"
	"github.com/YumikoKawaii/celine/basis/internal/hermes"
	"github.com/YumikoKawaii/celine/basis/internal/llm"
	"github.com/YumikoKawaii/celine/basis/internal/mneme"
	"github.com/YumikoKawaii/celine/basis/internal/rpc"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadServer()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := mneme.NewPool(ctx, cfg.DBDsn)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	rdb := mneme.NewRedis(cfg.RedisAddr)
	defer rdb.Close()

	var verifier *hermes.Verifier
	if cfg.JWTSecret != "" {
		verifier = hermes.NewVerifier(cfg.JWTSecret)
	}
	interceptor := hermes.NewAuthInterceptor(verifier)
	opts := connect.WithInterceptors(interceptor)

	tools := ergon.NewRegistry()
	tools.Register(ergon.NewWebSearch(cfg.BraveAPIKey))

	brain := llm.New(cfg.AnthropicKey, cfg.Model)
	convs := mneme.NewConversationStore(db)
	msgs := mneme.NewMessageStore(db, rdb)
	celineSvc := rpc.NewCelineService(agent.New(brain, agent.SystemPrompt(), convs, msgs, tools))

	var googleAuth *hermes.GoogleAuth
	var issuer *hermes.Issuer
	if cfg.GoogleClientID != "" {
		googleAuth = hermes.NewGoogleAuth(cfg.GoogleClientID, cfg.GoogleSecret)
		issuer = hermes.NewIssuer(cfg.JWTSecret)
	}
	hermesSvc := rpc.NewHermesService(googleAuth, issuer, mneme.NewClientStore(db))

	mux := http.NewServeMux()
	celinePath, celineHandler := celinev1connect.NewCelineHandler(celineSvc, opts)
	hermesPath, hermesHandler := celinev1connect.NewHermesHandler(hermesSvc, opts)
	mux.Handle(celinePath, celineHandler)
	mux.Handle(hermesPath, hermesHandler)

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: h2c.NewHandler(devCORS(mux), &http2.Server{}),
	}

	log.Printf("celine backend listening on %s", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
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
