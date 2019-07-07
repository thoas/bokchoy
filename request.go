package bokchoy

import (
	"context"
	"fmt"
)

type Request struct {
	ctx  context.Context
	Task *Task
}

func (r *Request) Context() context.Context {
	if r.ctx != nil {
		return r.ctx
	}
	return context.Background()
}

func (r *Request) WithContext(ctx context.Context) *Request {
	if ctx == nil {
		panic("nil context")
	}
	r2 := new(Request)
	*r2 = *r
	r2.ctx = ctx

	return r2
}

func (r Request) String() string {
	return fmt.Sprintf(
		"<Request task: %s>",
		r.Task,
	)
}
