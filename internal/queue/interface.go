// Package queue abstracts queue depth reads behind an interface so the
// reconciler can be tested without a real Redis instance.
package queue

import "context"

type Client interface {
	Depth(ctx context.Context, queueName string) (int64, error)
}

// Factory returns a Client for a given Redis address. Implementations may
// cache clients so repeated reconciles reuse connection pools.
type Factory interface {
	ForAddress(address string) Client
}
