package queue

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// RedisClient implements Client against a Redis list using LLEN
// One RedisClient is created per ScaledJob so each can point at a
// different redisAddress without sharing a connection pool.
type RedisClient struct {
	client *redis.Client
}

func NewRedisClient(address string) *RedisClient {
	return &RedisClient{
		client: redis.NewClient(&redis.Options{Addr: address}),
	}
}

// Depth returns the number of items in the named Redis list
// Redis LLEN is O(1), so calling it on every reconcile is safe
func (c *RedisClient) Depth(ctx context.Context, queueName string) (int64, error) {
	return c.client.LLen(ctx, queueName).Result()
}
