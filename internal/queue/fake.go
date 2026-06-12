package queue

import "context"

type FakeClient struct {
	DepthValue int64
	Err        error
	Calls      []string
}

func (f *FakeClient) Depth(_ context.Context, queueName string) (int64, error) {
	f.Calls = append(f.Calls, queueName)
	return f.DepthValue, f.Err
}
