package bokchoy_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thoas/bokchoy"
)

type noopconsumer struct {
}

func (c noopconsumer) Handle(r *bokchoy.Request) error {
	return nil
}

func TestConsumer_Consume(t *testing.T) {
	run(t, func(t *testing.T, s *suite) {
		is := assert.New(t)

		queue := s.bokchoy.Queue("tests.task.info")

		ticker := make(chan struct{})

		ctx := context.Background()
		queue.HandleFunc(func(r *bokchoy.Request) error {
			time.Sleep(time.Millisecond * 500)

			r.Task.Result = r.Task.Payload

			ticker <- struct{}{}

			return nil
		}, bokchoy.WithConcurrency(1))
		consumer := &noopconsumer{}
		queue.OnStart(consumer).
			OnComplete(consumer).
			OnFailure(consumer).
			OnSuccess(consumer)

		go func() {
			err := s.bokchoy.Run(ctx)
			is.NoError(err)
		}()

		task, err := queue.Publish(ctx, "world", bokchoy.WithSerializer(&bokchoy.JSONSerializer{}))
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
		is.Equal(task.Payload, task.Result)

		is.Equal(fmt.Sprintf("%.1f", task.ExecTime), "0.5")
	})
}

// nolint: govet
func TestConsumer_ConsumeRetries(t *testing.T) {
	run(t, func(t *testing.T, s *suite) {
		is := assert.New(t)

		queue := s.bokchoy.Queue("tests.task.error")

		maxRetries := 3

		ticker := make(chan struct{}, maxRetries)

		ctx := context.Background()
		queue.HandleFunc(func(r *bokchoy.Request) error {
			maxRetries--

			ticker <- struct{}{}

			return fmt.Errorf("An error occurred")
		})

		go func() {
			err := s.bokchoy.Run(ctx)
			is.NoError(err)
		}()

		task, err := queue.Publish(ctx, "error",
			bokchoy.WithMaxRetries(maxRetries), bokchoy.WithRetryIntervals([]time.Duration{
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
		is.True(task.IsStatusFailed())
		is.Equal(task.MaxRetries, 0)
	})
}

// nolint: govet,gosimple
func TestConsumer_ConsumeLong(t *testing.T) {
	run(t, func(t *testing.T, s *suite) {
		is := assert.New(t)

		queue := s.bokchoy.Queue("tests.task.long")

		ctx := context.Background()
		queue.HandleFunc(func(r *bokchoy.Request) error {
			time.Sleep(3 * time.Second)

			return nil
		})

		go func() {
			err := s.bokchoy.Run(ctx)
			is.NoError(err)
		}()

		task, err := queue.Publish(ctx, "long",
			bokchoy.WithTimeout(2*time.Second),
			bokchoy.WithMaxRetries(0))

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
		is.True(task.IsStatusCanceled())
	})
}
