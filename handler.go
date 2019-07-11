package bokchoy

// HandlerFunc is a handler to handle incoming tasks.
type HandlerFunc func(*Request) error

// Handle consumes the request.
func (s HandlerFunc) Handle(r *Request) error {
	return s(r)
}

// Handler is an interface to implement a task handler.
type Handler interface {
	Handle(*Request) error
}

func chain(middlewares []func(Handler) Handler, sub Handler) Handler {
	// Return ahead of time if there aren't any middlewares for the chain
	if len(middlewares) == 0 {
		return sub
	}

	// Wrap the end handler with the middleware chain
	h := middlewares[len(middlewares)-1](sub)
	for i := len(middlewares) - 2; i >= 0; i-- {
		h = middlewares[i](h)
	}

	return h
}
