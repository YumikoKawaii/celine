package taxis

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type Taxis struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Taxis {
	return &Taxis{rdb: rdb}
}

// Enqueue — consumer defines its own data.
func (t *Taxis) Enqueue(ctx context.Context, topic string, message interface{}) error {
	return t.rdb.LPush(ctx, topic, message).Err()
}

// Dequeue — consumer defines its own data.
// Returns (nil, nil) when the queue is empty (BRPOP timeout).
func (t *Taxis) Dequeue(ctx context.Context, topic string) (interface{}, error) {
	data, err := t.rdb.BRPop(ctx, 5*time.Second, topic).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}
