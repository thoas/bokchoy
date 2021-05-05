package bokchoy_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBokchoy_Queue(t *testing.T) {
	run(t, func(t *testing.T, s *suite) {
		is := assert.New(t)
		queue := s.bokchoy.Queue("tests.task.message")
		is.NotZero(queue)
		is.Equal(queue.Name(), "tests.task.message")
	})
}

func TestBokchoy_Flush(t *testing.T) {
	run(t, func(t *testing.T, s *suite) {
		is := assert.New(t)
		err := s.bokchoy.Flush(context.Background())
		is.NoError(err)
	})
}
