package bokchoy

import "fmt"

var (
	// ErrAttributeError is returned when an attribute is not found.
	ErrAttributeError = fmt.Errorf("Attribute error")

	// ErrTaskCanceled is returned when a task is canceled.
	ErrTaskCanceled = fmt.Errorf("Task canceled")
)
