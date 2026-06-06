package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/redis/go-redis/v9"

	"github.com/YumikoKawaii/celine/basis/internal/config"
	"github.com/YumikoKawaii/celine/basis/internal/graphe"
	"github.com/YumikoKawaii/celine/basis/internal/mneme"
	"github.com/YumikoKawaii/celine/basis/internal/taxis"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadWorker()
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
	embedder := graphe.NewOllamaClient(cfg.OllamaURL)
	w := graphe.NewWorker(rdb, embedder, store.Messages(), store.Memories(), tx)

	// §12.3: 1–2 concurrent workers caps concurrent embed calls and in-flight memory.
	const numWorkers = 2
	done := make(chan error, numWorkers)
	for range numWorkers {
		go func() {
			done <- w.Run(ctx)
		}()
	}

	<-ctx.Done()
	log.Println("graphe: shutting down, draining workers...")
	for range numWorkers {
		if err := <-done; err != nil {
			log.Printf("graphe: worker error: %v", err)
		}
	}
}
