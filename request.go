package bokchoy

import (
	"context"
	"fmt"
)

// Request is the bokchoy Request which will be handled
// by a subscriber handler.
type Request struct {
	ctx  context.Context
	Task *Task
}

// Context returns the context attached to the Request.
func (r *Request) Context() context.Context {
	if r.ctx != nil {
		return r.ctx
	}
	return context.Background()
}

// WithContext creates a new Request with a context
func (r *Request) WithContext(ctx context.Context) *Request {
	if ctx == nil {
		panic("nil context")
	}
	r2 := new(Request)
	*r2 = *r
	r2.ctx = ctx

	return r2
}

// String returns a string representation of a Request
func (r Request) String() string {
	return fmt.Sprintf(
		"<Request task: %s>",
		r.Task,
	)
}
