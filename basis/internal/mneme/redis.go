package mneme

import "github.com/redis/go-redis/v9"

// IndexQueue is the Redis list key shared between the server (LPUSH) and
// graphe worker (BRPOP).
const IndexQueue = "celine:graphe:queue"

func NewRedis(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{Addr: addr})
}
