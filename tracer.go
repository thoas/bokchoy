package bokchoy

import (
	"context"

	"github.com/thoas/bokchoy/logging"
)

type Tracer interface {
	Log(context.Context, string, error)
}

func NewTracerLogger(logger logging.Logger) Tracer {
	return &tracerLogger{logger}
}

type tracerLogger struct {
	logger logging.Logger
}

func (t tracerLogger) Log(ctx context.Context, msg string, err error) {
	t.logger.Error(ctx, msg, logging.Error(err))
}
