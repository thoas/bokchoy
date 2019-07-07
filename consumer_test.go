package bokchoy

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConsumer_Consume(t *testing.T) {
	run(t, func(t *testing.T, s *suite) {
		is := assert.New(t)

		queue := s.bokchoy.Queue("tests.task.info")

		ticker := make(chan struct{})

		ctx := context.Background()
		queue.SubscribeFunc(func(r *Request) error {
			time.Sleep(time.Millisecond * 500)

			ticker <- struct{}{}

			return nil
		})

		go func() {
			err := s.bokchoy.Run(ctx)
			is.NoError(err)
		}()

		task, err := queue.Publish(ctx, "world")
		is.NoError(err)

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		select {
		case <-ctx.Done():
			is.True(false)
		case <-ticker:
			is.True(true)
		}

		s.bokchoy.Stop(ctx)

		task, err = queue.Get(ctx, task.ID)
		is.NoError(err)
		is.True(task.IsStatusSucceeded())

		is.Equal(fmt.Sprintf("%.1f", task.ExecTime), "0.5")
	})
}

func TestConsumer_ConsumeRetries(t *testing.T) {
	run(t, func(t *testing.T, s *suite) {
		is := assert.New(t)

		queue := s.bokchoy.Queue("tests.task.error")

		maxRetries := 3

		ticker := make(chan struct{}, maxRetries)

		ctx := context.Background()
		queue.SubscribeFunc(func(r *Request) error {
			maxRetries--

			ticker <- struct{}{}

			return fmt.Errorf("An error occurred")
		})

		go func() {
			err := s.bokchoy.Run(ctx)
			is.NoError(err)
		}()

		task, err := queue.Publish(ctx, "error",
			WithMaxRetries(maxRetries), WithRetryIntervals([]time.Duration{
				1 * time.Second,
				2 * time.Second,
				3 * time.Second,
			}))
		is.NotZero(task)
		is.NoError(err)

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker:
				if maxRetries == 0 {
					return
				}
			}
		}

		is.Equal(maxRetries, 0)

		s.bokchoy.Stop(ctx)

		task, err = queue.Get(ctx, task.ID)
		is.NoError(err)
		is.Equal(task.Status, taskStatusFailed)
		is.Equal(task.MaxRetries, 0)
	})
}

func TestConsumer_ConsumeLong(t *testing.T) {
	run(t, func(t *testing.T, s *suite) {
		is := assert.New(t)

		queue := s.bokchoy.Queue("tests.task.long")

		ctx := context.Background()
		queue.SubscribeFunc(func(r *Request) error {
			time.Sleep(3 * time.Second)

			return nil
		})

		go func() {
			err := s.bokchoy.Run(ctx)
			is.NoError(err)
		}()

		task, err := queue.Publish(ctx, "long",
			WithTimeout(2*time.Second),
			WithMaxRetries(0))

		is.NotZero(task)
		is.NoError(err)

		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		for {
			select {
			case <-ctx.Done():
				return
			}
		}

		s.bokchoy.Stop(ctx)

		task, err = queue.Get(ctx, task.ID)
		is.NoError(err)
		is.Equal(task.Status, taskStatusCanceled)
	})
}
