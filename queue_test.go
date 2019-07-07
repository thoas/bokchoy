package bokchoy

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestQueue_ConsumeDelayed(t *testing.T) {
	run(t, func(t *testing.T, s *suite) {
		is := assert.New(t)
		ctx := context.Background()

		queue := s.bokchoy.Queue("tests.task.message")
		ticker := make(chan struct{})

		queue.SubscribeFunc(func(r *Request) error {
			ticker <- struct{}{}

			return nil
		})

		go func() {
			err := s.bokchoy.Run(ctx)
			is.NoError(err)
		}()

		task, err := queue.Publish(ctx, "world", WithCountdown(2*time.Second))
		is.NotZero(task)
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
	})
}

func TestQueue_Save(t *testing.T) {
	run(t, func(t *testing.T, s *suite) {
		is := assert.New(t)
		ctx := context.Background()
		queue := s.bokchoy.Queue("tests.task.message")
		task1, err := queue.Publish(ctx, "hello", WithTTL(5*time.Second))
		is.NotZero(task1)
		is.NoError(err)

		task1.MarkAsSucceeded()
		queue.Save(ctx, task1)

		task2, err := queue.Get(ctx, task1.ID)
		is.NotZero(task2)
		is.NoError(err)
		is.Equal(task2.Status, taskStatusSucceeded)
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
		is.Equal(stats.Total, 1)
		is.Equal(stats.Direct, 1)
		is.Equal(stats.Delayed, 0)
		is.Nil(err)
		is.NotZero(task1)
		is.Equal(task1.Name, queue.name)

		err = queue.Empty(ctx)
		is.NoError(err)

		task2, err := queue.Publish(ctx, "hello", WithCountdown(60*time.Second))
		is.NoError(err)
		stats, err = queue.Count(ctx)
		is.NoError(err)
		is.NotZero(task2.ETA)

		is.Equal(stats.Delayed, 1)
		is.Equal(stats.Total, 1)
		is.Equal(stats.Direct, 0)
		is.NotZero(task2)
		is.Equal(task2.Name, queue.name)

		task3, err := queue.Get(ctx, task2.ID)
		is.NoError(err)
		is.NotZero(task3)
		is.NotZero(task3.ETA)
		is.Equal(task3.ETA.Unix(), task2.ETA.Unix())
	})
}
