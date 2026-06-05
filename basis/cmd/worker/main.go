package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/YumikoKawaii/celine/basis/internal/graphe"
	"github.com/YumikoKawaii/celine/basis/internal/mneme"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := mneme.NewPool(ctx, mustEnv("CELINE_DB_DSN"))
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	rdb := mneme.NewRedis(mustEnv("CELINE_REDIS_ADDR"))
	defer rdb.Close()

	embedder := graphe.NewOllamaClient(os.Getenv("OLLAMA_URL"))
	w := graphe.NewWorker(db, rdb, embedder)

	// §12.3: 1–2 concurrent workers caps concurrent embed calls and in-flight memory.
	const numWorkers = 2
	done := make(chan struct{}, numWorkers)
	for range numWorkers {
		go func() {
			defer func() { done <- struct{}{} }()
			w.Run(ctx)
		}()
	}

	<-ctx.Done()
	log.Println("graphe: shutting down, draining workers...")
	for range numWorkers {
		<-done
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s is required", key)
	}
	return v
}
