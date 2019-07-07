package bokchoy

// SubscriberHandler is a handler to handle incoming tasks.
type SubscriberFunc func(*Request) error

func (s SubscriberFunc) Consume(r *Request) error {
	return s(r)
}

// Subscriber is an interface to implement a task handler.
type Subscriber interface {
	Consume(*Request) error
}
