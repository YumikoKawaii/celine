package graphe

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/YumikoKawaii/celine/basis/internal/mneme"
)

// memoryIndexer is the subset of mneme.MemoryIndexRepo this worker needs.
// Defined here (consumer package) per the project's interface convention.
type memoryIndexer interface {
	Insert(ctx context.Context, job mneme.IndexJob, embedding []float32) error
}

// Worker consumes index jobs from the Redis queue, embeds each message,
// and writes the resulting vector into memory_index (§12).
type Worker struct {
	rdb      *redis.Client
	embedder Embedder
	indexer  memoryIndexer
}

func NewWorker(rdb *redis.Client, embedder Embedder, indexer memoryIndexer) *Worker {
	return &Worker{rdb: rdb, embedder: embedder, indexer: indexer}
}

// Run blocks, consuming jobs until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	log.Println("graphe: worker started")
	for {
		select {
		case <-ctx.Done():
			log.Println("graphe: worker stopped")
			return
		default:
		}

		// BRPOP with a short timeout so we can check ctx.Done() regularly.
		res, err := w.rdb.BRPop(ctx, 5*time.Second, mneme.IndexQueue).Result()
		if err == redis.Nil {
			continue // timeout — no jobs, loop and recheck context
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("graphe: queue error: %v", err)
			continue
		}

		// res[0] = key, res[1] = payload
		var job mneme.IndexJob
		if err := json.Unmarshal([]byte(res[1]), &job); err != nil {
			log.Printf("graphe: malformed job: %v", err)
			continue
		}

		if err := w.process(ctx, job); err != nil {
			log.Printf("graphe: failed %s: %v", job.MessageID, err)
		}
	}
}

func (w *Worker) process(ctx context.Context, job mneme.IndexJob) error {
	vec, err := w.embedder.Embed(ctx, job.Content)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}
	return w.indexer.Insert(ctx, job, vec)
}
