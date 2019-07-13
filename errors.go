package bokchoy

import "fmt"

var (
	// ErrAttributeError is returned when an attribute is not found.
	ErrAttributeError = fmt.Errorf("Attribute error")

	// ErrTaskCanceled is returned when a task is canceled.
	ErrTaskCanceled = fmt.Errorf("Task canceled")

	// ErrTaskNotFound is returned when a task is not found.
	ErrTaskNotFound = fmt.Errorf("Task not found")

	// ErrNoQueueToRun is returned when no queue has been found to run.
	ErrNoQueueToRun = fmt.Errorf("No queue to run")
)
