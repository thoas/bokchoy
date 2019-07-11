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
	handler     Subscriber
	middlewares []func(Subscriber) Subscriber
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

	if c.closed == false {
		c.wg.Done()
		c.closed = true
	}

	c.logger.Debug(ctx, "Stopped")
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

	go func() {
		err = chain(c.middlewares, c.handler).Consume(r)
		if err != nil {
			conn <- result{
				err: err,
			}
		}

		conn <- result{}
	}()

	select {
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

func (c *consumer) handleError(ctx context.Context, task *Task, err error) error {
	if err == nil {
		task.MarkAsSucceeded()

		c.logger.Debug(ctx, "Task marked as succeeded", logging.Object("task", task))

		err = c.queue.Save(ctx, task)
		if err != nil {
			return errors.Wrapf(err, "unable to handle error %s", task)
		}

		return nil
	}

	if err == ErrTaskCanceled {
		task.MarkAsCanceled()
	} else {
		task.MarkAsFailed(err)
	}

	if task.MaxRetries == 0 {
		c.logger.Debug(ctx, "Task marked as failed: no retry", logging.Object("task", task))

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

func (c *consumer) Consume(r *Request) error {
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

		c.queue.fireEvents(r)

		err = c.handleTask(ctx, r)

		err = c.handleError(ctx, task, err)
		if err != nil {
			return err
		}
	}

	return c.queue.fireEvents(r)
}

func (c *consumer) start(ctx context.Context) {
	c.wg.Add(1)

	c.consume(ctx)

	c.logger.Debug(ctx, "Started")
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
				err = c.Consume(req)
				if err != nil {
					c.tracer.Log(ctx, "Receive error when handling", err)
				}
			}
		}
	}()
}
