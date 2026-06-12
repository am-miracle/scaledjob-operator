// Package queue abstracts queue depth reads behind an interface so the
// reconciler can be tested without a real Redis instance.
package queue

import "context"

type Client interface {
	Depth(ctx context.Context, queueName string) (int64, error)
}
