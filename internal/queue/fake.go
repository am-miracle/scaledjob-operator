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

// FakeFactory always returns the same FakeClient so tests can inspect
// calls and set return values without caring about the Redis address.
type FakeFactory struct {
	Client *FakeClient
}

func (f *FakeFactory) ForAddress(_ string) Client {
	return f.Client
}
