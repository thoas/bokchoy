package bokchoy_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thoas/bokchoy"
)

func TestTask_Serialize(t *testing.T) {
	is := assert.New(t)
	task := bokchoy.NewTask("task.message", "hello")
	is.NotZero(task)

	serializer := bokchoy.JSONSerializer{}

	results, err := task.Serialize(serializer)
	is.Nil(err)
	is.NotZero(results)

	is.True(task.IsStatusWaiting())
	is.Equal(task.Name, "task.message")
	is.NotZero(task.Payload)
	is.NotZero(task.PublishedAt)

	task2, err := bokchoy.TaskFromPayload(results, serializer)

	is.Nil(err)
	is.NotZero(task2)
}

func TestTask_Finished(t *testing.T) {
	is := assert.New(t)
	task := bokchoy.NewTask("task.message", "hello")
	is.False(task.Finished())
	task.MarkAsSucceeded()
	is.True(task.Finished())
	task.MarkAsFailed(nil)
	task.MaxRetries < 0
	is.True(task.Finished())
	task.MaxRetries = 3
	is.False(task.Finished())
}
