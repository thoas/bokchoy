package logging

import (
	"context"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// DefaultLogger is the default logger.
var DefaultLogger = NewNopLogger()

// Field is the logger field.
type Field = zapcore.Field

// ObjectEncoder is the logger representation of a structure.
type ObjectEncoder = zapcore.ObjectEncoder

// Logger is the standart logger interface.
type Logger interface {
	Panic(context.Context, string, ...Field)
	Info(context.Context, string, ...Field)
	Error(context.Context, string, ...Field)
	Debug(context.Context, string, ...Field)
	Sync() error
	With(fields ...Field) Logger
}

// String appends a string field.
func String(k, v string) Field {
	return zap.String(k, v)
}

// Duration appends a duration field.
func Duration(k string, d time.Duration) Field {
	return zap.Duration(k, d)
}

// Float64 appends a float64 field.
func Float64(key string, val float64) Field {
	return zap.Float64(key, val)
}

// Time appends a time field.
func Time(key string, val time.Time) Field {
	return zap.Time(key, val)
}

// Int appends an int field.
func Int(k string, i int) Field {
	return zap.Int(k, i)
}

// Int64 appends an int64 field.
func Int64(k string, i int64) Field {
	return zap.Int64(k, i)
}

// Error appends an error field.
func Error(v error) Field {
	return zap.Error(v)
}

// Object appends an object field with implements ObjectMarshaler interface.
func Object(key string, val zapcore.ObjectMarshaler) Field {
	return zap.Object(key, val)
}

// NewProductionLogger initializes a production logger.
func NewProductionLogger() (Logger, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}

	return &wrapLogger{logger}, nil
}

// NewDevelopmentLogger initializes a development logger.
func NewDevelopmentLogger() (Logger, error) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}

	return &wrapLogger{logger}, nil
}

// NewNopLogger initializes a noop logger.
func NewNopLogger() Logger {
	return &wrapLogger{zap.NewNop()}
}

type wrapLogger struct {
	*zap.Logger
}

// With creates a child logger and adds structured context to it. Fields added
// to the child don't affect the parent, and vice versa.
func (l wrapLogger) With(fields ...Field) Logger {
	return &wrapLogger{l.Logger.With(fields...)}
}

// Panic logs a message at PanicLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
//
// The logger then panics, even if logging at PanicLevel is disabled.
func (l wrapLogger) Panic(ctx context.Context, msg string, fields ...Field) {
	l.Logger.Panic(msg, fields...)
}

// Info logs a message at InfoLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func (l wrapLogger) Info(ctx context.Context, msg string, fields ...Field) {
	l.Logger.Info(msg, fields...)

}

// Error logs a message at ErrorLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func (l wrapLogger) Error(ctx context.Context, msg string, fields ...Field) {
	l.Logger.Error(msg, fields...)
}

// Debug logs a message at DebugLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func (l wrapLogger) Debug(ctx context.Context, msg string, fields ...Field) {
	l.Logger.Debug(msg, fields...)
}
