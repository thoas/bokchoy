package bokchoy

// SubscriberFunc is a handler to handle incoming tasks.
type SubscriberFunc func(*Request) error

// Consume consumes the request.
func (s SubscriberFunc) Consume(r *Request) error {
	return s(r)
}

// Subscriber is an interface to implement a task handler.
type Subscriber interface {
	Consume(*Request) error
}

func chain(middlewares []func(Subscriber) Subscriber, sub Subscriber) Subscriber {
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
