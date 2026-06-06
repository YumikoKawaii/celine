package graphe

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/redis/go-redis/v9"

	"github.com/YumikoKawaii/celine/basis/internal/arche"
	"github.com/YumikoKawaii/celine/basis/internal/mneme"
)

type embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type messages interface {
	Get(ctx context.Context, scope mneme.Scope) (mneme.Message, error)
}

type memories interface {
	Insert(ctx context.Context, memory mneme.Memory, embedding []float32) error
}

type queue interface {
	Dequeue(ctx context.Context, topic string) (interface{}, error)
}

// Worker consumes index jobs from the Redis queue, embeds each message,
// and writes the resulting vector into memories (§12).
type Worker struct {
	rdb      *redis.Client
	embedder embedder
	messages messages
	memories memories
	queue    queue
}

func NewWorker(rdb *redis.Client, embedder embedder, messages messages, memories memories, queue queue) *Worker {
	return &Worker{
		rdb:      rdb,
		embedder: embedder,
		messages: messages,
		memories: memories,
		queue:    queue,
	}
}

// Run blocks, consuming jobs until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	log.Println("graphe: worker started")
	for {
		select {
		case <-ctx.Done():
			log.Println("graphe: worker stopped")
			return nil
		default:
		}

		data, err := w.queue.Dequeue(ctx, arche.GrapheQueue)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("graphe: dequeue error: %v", err)
			continue
		}
		if data == nil {
			continue // timeout — no jobs
		}

		// BRPop returns []string{key, value}; value is the message id as string.
		parts, ok := data.([]string)
		if !ok || len(parts) < 2 {
			log.Printf("graphe: unexpected payload type %T", data)
			continue
		}
		messageId, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			log.Printf("graphe: malformed payload %q: %v", parts[1], err)
			continue
		}

		if err := w.process(ctx, messageId); err != nil {
			return err
		}
	}
}

func (w *Worker) process(ctx context.Context, messageId int64) error {
	message, err := w.messages.Get(ctx, mneme.KataMessage{Id: messageId})
	if err != nil {
		return fmt.Errorf("fetch message %d: %w", messageId, err)
	}
	vec, err := w.embedder.Embed(ctx, message.Content)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}
	return w.memories.Insert(ctx, mneme.Memory{MessageId: messageId}, vec)
}
