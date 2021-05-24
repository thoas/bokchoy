package bokchoy

import (
	"context"
	"fmt"
	"sync"

	"github.com/thoas/bokchoy/logging"

	"github.com/pkg/errors"
)

type consumer struct {
	name        string
	handler     Handler
	middlewares []func(Handler) Handler
	queue       *Queue
	serializer  Serializer
	logger      logging.Logger
	tracer      Tracer
	wg          *sync.WaitGroup
	closed      bool
	mu          *sync.Mutex
}

func (c *consumer) stop(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.closed {
		c.wg.Done()
		c.closed = true
	}

	c.logger.Debug(ctx, fmt.Sprintf("Stopped %s", c))
}

func (c *consumer) String() string {
	return c.name
}

func (c *consumer) handleTask(ctx context.Context, r *Request) error {
	var (
		err  error
		task = r.Task
	)

	timeout, cancel := context.WithTimeout(ctx, task.Timeout)
	defer cancel()

	type result struct {
		err error
	}

	conn := make(chan result)

	// we execute the complete request lifecycle in a goroutine
	// to handle timeout and retrieve the worker back if the executing
	// time a task is higher than timeout.
	go func() {
		defer func() {
			conn <- result{
				err: err,
			}
		}()

		middlewares := c.middlewares
		middlewares = append(middlewares, c.handleRequest)

		// chain wraps the n middleware with its n-1
		// we keep the reference of the final request since it contains
		// a modified context by middlewares.
		err = chain(middlewares, HandlerFunc(func(req *Request) error {
			*r = *req

			// execute the underlying handler.
			return c.handler.Handle(req)
		})).Handle(r)

		// retrieve the error in the context added
		// by the recoverer middleware.
		if err == nil {
			err = GetContextError(r.Context())
		}
	}()

	select {
	// timeout done, we have to cancel the task.
	case <-timeout.Done():
		c.logger.Debug(ctx, "Task canceled by timeout", logging.Object("task", task))
		err = ErrTaskCanceled
	case res := <-conn:
		err = res.err
	}

	if err != nil {
		return errors.Wrapf(err, "unable to handle %s", task)
	}

	return nil
}

// handleError can be called twice since it's called inside Handle
// and in handleRequest.
func (c *consumer) handleError(ctx context.Context, task *Task, err error) error {
	// Why do we have to call it twice?
	// A panicking task should be marked as failed, when it's panicking
	// the first handleError is skipped.
	if !task.IsStatusProcessing() {
		return nil
	}

	if err == nil {
		task.MarkAsSucceeded()

		c.logger.Debug(ctx, "Task marked as succeeded", logging.Object("task", task))

		err = c.queue.Save(ctx, task)
		if err != nil {
			return errors.Wrapf(err, "unable to handle error %s", task)
		}

		return nil
	}

	c.tracer.Log(ctx, "Received an error when handling task", errors.Wrapf(err, "unable to handle task %s", task))

	if errors.Cause(err) == ErrTaskCanceled {
		task.MarkAsCanceled()

		c.logger.Debug(ctx, "Task marked as canceled", logging.Object("task", task))
	} else {
		task.MarkAsFailed(err)
	}

	if task.MaxRetries == 0 {
		c.logger.Debug(ctx, "Task marked as failed: no retry", logging.Object("task", task))

		task.MaxRetries -= 1

		err = c.queue.Save(ctx, task)
		if err != nil {
			return errors.Wrapf(err, "unable to handle error %s", task)
		}

		return nil
	}

	task.MaxRetries -= 1
	task.ETA = task.RetryETA()

	c.logger.Debug(ctx, fmt.Sprintf("Task marked as failed: retrying in %s...", task.ETADisplay()),
		logging.Object("task", task))

	err = c.queue.PublishTask(ctx, task)

	if err != nil {
		return errors.Wrapf(err, "unable to handle error %s", task)
	}

	return nil
}

func (c *consumer) handleRequest(next Handler) Handler {
	return HandlerFunc(func(req *Request) error {
		var (
			ctx = req.Context()
			err = next.Handle(req)
		)

		return c.handleError(ctx, req.Task, err)
	})
}

func (c *consumer) Handle(r *Request) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var (
		task = r.Task
		err  error
	)

	ctx := r.Context()

	c.logger.Debug(ctx, "Task received", logging.Object("task", task))

	if task.IsStatusCanceled() {
		c.logger.Debug(ctx, "Task has been previously canceled", logging.Object("task", task))
	} else {
		task.MarkAsProcessing()

		c.logger.Debug(ctx, "Task processing...", logging.Object("task", task))

		err = c.queue.Save(ctx, task)
		if err != nil {
			return err
		}

		err = c.queue.fireEvents(r)
		if err != nil {
			return err
		}

		err = c.handleTask(ctx, r)

		err = c.handleError(ctx, task, err)
		if err != nil {
			return err
		}

		funcs := GetContextAfterRequestFuncs(r.Context())
		for i := range funcs {
			funcs[i]()
		}
	}

	return c.queue.fireEvents(r)
}

func (c *consumer) start(ctx context.Context) {
	c.wg.Add(1)

	c.consume(ctx)

	c.logger.Debug(ctx, fmt.Sprintf("Started %s", c))
}

func (c *consumer) isClosed() bool {
	c.mu.Lock()
	closed := c.closed
	defer c.mu.Unlock()

	return closed
}

func (c *consumer) consume(ctx context.Context) {
	go func() {
		for {
			ctx := context.Background()

			if c.isClosed() {
				return
			}

			tasks, err := c.queue.Consume(ctx)
			if err != nil {
				c.tracer.Log(ctx, "Receive error from publisher", err)
			}

			if len(tasks) == 0 {
				continue
			}

			c.logger.Debug(ctx, "Received tasks to consume", logging.Int("tasks_count", len(tasks)))

			for i := range tasks {
				task := tasks[i]

				req := &Request{Task: task}
				err = c.Handle(req.WithContext(WithContextTask(req.Context(), task)))
				if err != nil {
					c.tracer.Log(ctx, "Receive error when handling", err)
				}
			}
		}
	}()
}

var _ Handler = (*consumer)(nil)
