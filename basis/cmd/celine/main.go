package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"connectrpc.com/connect"
	"github.com/redis/go-redis/v9"
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
	"github.com/YumikoKawaii/celine/basis/internal/taxis"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadServer()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := mneme.NewDB(cfg.DBDsn)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer func() { _ = mneme.CloseDB(db) }()

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer rdb.Close()

	store := mneme.NewMneme(db)
	tx := taxis.New(rdb)

	var verifier *hermes.Verifier
	if cfg.JWTSecret != "" {
		verifier = hermes.NewVerifier(cfg.JWTSecret)
	}
	interceptor := hermes.NewAuthInterceptor(verifier)
	opts := connect.WithInterceptors(interceptor)

	tools := ergon.NewRegistry()
	if cfg.BraveAPIKey != "" {
		tools.Register(ergon.NewWebSearch(cfg.BraveAPIKey))
	}

	brain := llm.New(cfg.AnthropicKey, cfg.Model, cfg.MaxTokens)
	celineSvc := rpc.NewCeline(
		agent.New(brain, agent.SystemPrompt(), store.Prosopons(), store.Conversations(), store.Messages(), tx, tools),
		cfg.DebounceDuration,
	)

	var googleAuth *hermes.GoogleAuth
	var issuer *hermes.Issuer
	if cfg.GoogleClientID != "" {
		googleAuth = hermes.NewGoogleAuth(cfg.GoogleClientID, cfg.GoogleSecret)
		issuer = hermes.NewIssuer(cfg.JWTSecret, cfg.TokenTTL)
	}
	hermesSvc := rpc.NewHermes(googleAuth, issuer, store.Prosopons())

	mux := http.NewServeMux()
	celinePath, celineHandler := celinev1connect.NewCelineHandler(celineSvc, opts)
	hermesPath, hermesHandler := celinev1connect.NewHermesHandler(hermesSvc, opts)
	mux.Handle(celinePath, celineHandler)
	mux.Handle(hermesPath, hermesHandler)

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	// Shut down gracefully when the signal context is cancelled.
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	log.Printf("celine backend listening on %s", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
