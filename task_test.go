package bokchoy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTask_Serialize(t *testing.T) {
	is := assert.New(t)
	task := NewTask("task.message", "hello")
	is.NotZero(task)

	serializer := JSONSerializer{}

	results, err := task.Serialize(serializer)
	is.Nil(err)
	is.NotZero(results)

	is.Equal(task.Status, taskStatusWaiting)
	is.Equal(task.Name, "task.message")
	is.NotZero(task.Payload)
	is.NotZero(task.PublishedAt)

	task2, err := taskFromPayload(results, serializer)

	is.Nil(err)
	is.NotZero(task2)
}

func TestTask_Finished(t *testing.T) {
	is := assert.New(t)
	task := NewTask("task.message", "hello")
	is.False(task.Finished())
	task.Status = taskStatusSucceeded
	is.True(task.Finished())
	task.Status = taskStatusFailed
	task.MaxRetries = 0
	is.True(task.Finished())
	task.MaxRetries = 3
	is.False(task.Finished())
}
