package queue

import (
	"context"
	"errors"
	"sync"

	"github.com/redis/go-redis/v9"
)

// RedisClient implements Client against a Redis list using LLEN.
type RedisClient struct {
	client *redis.Client
}

func NewRedisClient(address string) *RedisClient {
	return &RedisClient{
		client: redis.NewClient(&redis.Options{Addr: address}),
	}
}

// Depth returns the number of items in the named Redis list.
// Redis LLEN is O(1), so calling it on every reconcile is safe.
func (c *RedisClient) Depth(ctx context.Context, queueName string) (int64, error) {
	return c.client.LLen(ctx, queueName).Result()
}

func (c *RedisClient) Close() error {
	return c.client.Close()
}

// RedisFactory caches one RedisClient per address and closes them on shutdown.
type RedisFactory struct {
	mu      sync.Mutex
	clients map[string]*RedisClient
}

func (f *RedisFactory) ForAddress(address string) Client {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.clients == nil {
		f.clients = make(map[string]*RedisClient)
	}
	if client, ok := f.clients[address]; ok {
		return client
	}

	client := NewRedisClient(address)
	f.clients[address] = client
	return client
}

func (f *RedisFactory) Start(ctx context.Context) error {
	<-ctx.Done()
	return f.Close()
}

func (f *RedisFactory) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var closeErr error
	for address, client := range f.clients {
		closeErr = errors.Join(closeErr, client.Close())
		delete(f.clients, address)
	}
	return closeErr
}
