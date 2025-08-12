package pool

import "context"

type Result struct {
	Value interface{}
	Err   error
}

type Future struct {
	ch <-chan Result
}

func (f Future) Get(ctx context.Context) (interface{}, error) {
	select {
	case res := <-f.ch:
		return res.Value, res.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
