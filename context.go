package bokchoy

import (
	"context"
)

type contextKey struct {
	name string
}

// AfterRequestFunc is a function which will execute after the request
type AfterRequestFunc func()

func (k *contextKey) String() string {
	return "bokchoy context value " + k.name
}

var (
	// ErrorCtxKey is the context.Context key to store
	// the recovered error from the middleware
	ErrorCtxKey = &contextKey{"Error"}

	// TaskCtxKey is the context.Context key to store
	// the task from the middleware
	TaskCtxKey = &contextKey{"Task"}

	// AfterRequestCtxKey is the context.Context to store
	// functions to execute after the request
	AfterRequestCtxKey = &contextKey{"AfterRequest"}
)

// WithContextTask sets the in-context task for a request.
func WithContextTask(ctx context.Context, task *Task) context.Context {
	ctx = context.WithValue(ctx, TaskCtxKey, task)
	return ctx
}

// GetContextTask returns the in-context task for a request.
func GetContextTask(ctx context.Context) *Task {
	err, _ := ctx.Value(TaskCtxKey).(*Task)
	return err
}

// WithContextError sets the in-context error for a request.
func WithContextError(ctx context.Context, err error) context.Context {
	ctx = context.WithValue(ctx, ErrorCtxKey, err)
	return ctx
}

// GetContextError returns the in-context recovered error for a request.
func GetContextError(ctx context.Context) error {
	err, _ := ctx.Value(ErrorCtxKey).(error)
	return err
}

// GetContextAfterRequestFuncs returns the registered functions
// which will execute after the request
func GetContextAfterRequestFuncs(ctx context.Context) []AfterRequestFunc {
	funcs, _ := ctx.Value(AfterRequestCtxKey).([]AfterRequestFunc)
	if funcs == nil {
		return []AfterRequestFunc{}
	}

	return funcs
}

// WithContextAfterRequestFunc registers a new function to be executed
// after the request
func WithContextAfterRequestFunc(ctx context.Context, f AfterRequestFunc) context.Context {
	funcs := append(GetContextAfterRequestFuncs(ctx), f)
	return context.WithValue(ctx, AfterRequestCtxKey, funcs)
}
