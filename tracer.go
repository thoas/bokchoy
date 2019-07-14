package bokchoy

import (
	"context"

	"github.com/thoas/bokchoy/logging"
)

// DefaultTracer is the default tracer.
var DefaultTracer = NewLoggerTracer(logging.DefaultLogger)

// Tracer is a component used to trace errors.
type Tracer interface {
	Log(context.Context, string, error)
}

// NewLoggerTracer initializes a new Tracer instance.
func NewLoggerTracer(logger logging.Logger) Tracer {
	return &loggerTracer{logger}
}

type loggerTracer struct {
	logger logging.Logger
}

func (t loggerTracer) String() string {
	return "logger"
}

// Log logs the error.
func (t loggerTracer) Log(ctx context.Context, msg string, err error) {
	t.logger.Error(ctx, msg, logging.Error(err))
}
