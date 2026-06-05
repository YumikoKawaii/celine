package graphe

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/YumikoKawaii/celine/basis/internal/mneme"
)

type Worker struct {
	db       *pgxpool.Pool
	rdb      *redis.Client
	embedder Embedder
}

func NewWorker(db *pgxpool.Pool, rdb *redis.Client, embedder Embedder) *Worker {
	return &Worker{db: db, rdb: rdb, embedder: embedder}
}

// Run blocks, consuming jobs from the index queue until ctx is cancelled.
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
			continue // timeout, no jobs
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

	_, err = w.db.Exec(ctx,
		`INSERT INTO memory_index (owner_sub, message_id, role, content, embedding)
		 VALUES ($1, $2, $3, $4, $5::vector)
		 ON CONFLICT (message_id) DO NOTHING`,
		job.OwnerSub, job.MessageID, job.Role, job.Content, vecLiteral(vec),
	)
	return err
}

// vecLiteral formats a float32 slice as a Postgres vector literal: '[x,y,...]'
func vecLiteral(v []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(f), 'f', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}
