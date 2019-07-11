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

	// AfterRequestCtxKey is the context.Context to store
	// functions to execute after the request
	AfterRequestCtxKey = &contextKey{"AfterRequest"}
)

// GetError returns the in-context recovered error for a request.
func GetError(r *Request) error {
	err, _ := r.Context().Value(ErrorCtxKey).(error)
	return err
}

// WithError sets the in-context LogEntry for a request.
func WithError(r *Request, err error) *Request {
	r = r.WithContext(context.WithValue(r.Context(), ErrorCtxKey, err))
	return r
}

// GetAfterRequestFuncs returns the registered functions
// which will execute after the request
func GetAfterRequestFuncs(r *Request) []AfterRequestFunc {
	funcs, _ := r.Context().Value(AfterRequestCtxKey).([]AfterRequestFunc)
	if funcs == nil {
		return []AfterRequestFunc{}
	}

	return funcs
}

// WithAfterRequestFunc registers a new function to be executed
// after the request
func WithAfterRequestFunc(r *Request, f AfterRequestFunc) *Request {
	funcs := append(GetAfterRequestFuncs(r), f)
	ctx := context.WithValue(r.Context(), AfterRequestCtxKey, funcs)
	return r.WithContext(ctx)
}
