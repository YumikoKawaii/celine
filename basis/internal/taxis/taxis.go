package taxis

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

// IndexQueue is the Redis list key shared between the agent (LPUSH) and
// the graphe worker (BRPOP).
const IndexQueue = "celine:graphe:queue"

// IndexJob is the payload pushed onto IndexQueue.
// graphe deserialises it to embed the message content and write to memories.
type IndexJob struct {
	MessageID int64  `json:"message_id"`
	Content   string `json:"content"`
}

type Taxis struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Taxis {
	return &Taxis{rdb: rdb}
}

func (t *Taxis) Enqueue(ctx context.Context, topic string, message interface{}) error {
	b, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return t.rdb.LPush(ctx, topic, b).Err()
}
