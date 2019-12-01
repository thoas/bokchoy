package bokchoy_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thoas/bokchoy"
)

func TestQueue_Consumer(t *testing.T) {
	run(t, func(t *testing.T, s *suite) {
		is := assert.New(t)
		queue := s.bokchoy.Queue("tests.task.message")
		queue.HandleFunc(func(r *bokchoy.Request) error {
			return nil
		})
		is.NotZero(queue.Consumer())
	})
}

func TestQueue_Cancel(t *testing.T) {
	run(t, func(t *testing.T, s *suite) {
		is := assert.New(t)
		ctx := context.Background()
		queue := s.bokchoy.Queue("tests.task.message")
		task1, err := queue.Publish(ctx, "hello", bokchoy.WithTTL(10*time.Second))
		is.NotZero(task1)
		is.NoError(err)
		task2, err := queue.Cancel(ctx, task1.ID)
		is.NotZero(task2)
		is.NoError(err)
		is.True(task2.IsStatusCanceled())
	})
}

func TestQueue_Save(t *testing.T) {
	run(t, func(t *testing.T, s *suite) {
		is := assert.New(t)
		ctx := context.Background()
		queue := s.bokchoy.Queue("tests.task.message")
		task1, err := queue.Publish(ctx, "hello", bokchoy.WithTTL(10*time.Second))
		is.NotZero(task1)
		is.NoError(err)

		task1.MarkAsSucceeded()
		err = queue.Save(ctx, task1)
		is.NoError(err)

		task2, err := queue.Get(ctx, task1.ID)
		is.NotZero(task2)
		is.NoError(err)
		is.True(task2.IsStatusSucceeded())
		is.NotZero(task2.ProcessedAt)
		is.NotZero(task2.ExecTime)
	})
}

func TestQueue_Publish(t *testing.T) {
	run(t, func(t *testing.T, s *suite) {
		is := assert.New(t)
		ctx := context.Background()

		queue := s.bokchoy.Queue("tests.task.message")
		task1, err := queue.Publish(ctx, "hello")
		is.NoError(err)
		stats, err := queue.Count(ctx)
		is.NoError(err)
		is.Equal(1, stats.Total)
		is.Equal(1, stats.Direct)
		is.Nil(err)
		is.NotZero(task1)
		is.Equal(task1.Name, queue.Name())

		err = queue.Empty(ctx)
		is.NoError(err)

		task2, err := queue.Publish(ctx, "hello", bokchoy.WithCountdown(60*time.Second))
		is.NoError(err)
		stats, err = queue.Count(ctx)
		is.NoError(err)
		is.NotZero(task2.ETA)

		is.Equal(1, stats.Total)
		is.Equal(0, stats.Direct)
		is.NotZero(task2)
		is.Equal(task2.Name, queue.Name())

		task3, err := queue.Get(ctx, task2.ID)
		is.NoError(err)
		is.NotZero(task3)
		is.NotZero(task3.ETA)
		is.Equal(task3.ETA.Unix(), task2.ETA.Unix())
	})
}

type consumer struct {
	ticker chan struct{}
}

func (c consumer) Handle(r *bokchoy.Request) error {
	c.ticker <- struct{}{}

	return nil
}

func TestQueue_ConsumeDelayed(t *testing.T) {
	run(t, func(t *testing.T, s *suite) {
		is := assert.New(t)
		ctx := context.Background()

		consumer := &consumer{
			ticker: make(chan struct{}),
		}

		queueName := "tests.task.message"

		s.bokchoy.Handle(queueName, consumer)

		go func() {
			err := s.bokchoy.Run(ctx)
			is.NoError(err)
		}()

		task, err := s.bokchoy.Publish(ctx, queueName, "world", bokchoy.WithCountdown(2*time.Second))
		is.NotZero(task)
		is.NoError(err)

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		select {
		case <-ctx.Done():
			is.True(false)
		case <-consumer.ticker:
			is.True(true)
		}

		s.bokchoy.Stop(ctx)
	})
}
