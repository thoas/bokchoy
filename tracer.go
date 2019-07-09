package bokchoy

import (
	"context"

	"github.com/thoas/bokchoy/logging"
)

// DefaultTracer is the default tracer.
var DefaultTracer = NewTracerLogger(logging.DefaultLogger)

// Tracer is a component used to trace errors.
type Tracer interface {
	Log(context.Context, string, error)
}

// NewTracerLogger initializes a new Tracer instance.
func NewTracerLogger(logger logging.Logger) Tracer {
	return &tracerLogger{logger}
}

type tracerLogger struct {
	logger logging.Logger
}

// Log logs the error.
func (t tracerLogger) Log(ctx context.Context, msg string, err error) {
	t.logger.Error(ctx, msg, logging.Error(err))
}
