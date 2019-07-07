package logging

import (
	"context"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Field = zapcore.Field
type ObjectEncoder = zapcore.ObjectEncoder

type Logger interface {
	Panic(context.Context, string, ...Field)
	Info(context.Context, string, ...Field)
	Error(context.Context, string, ...Field)
	Debug(context.Context, string, ...Field)
	Sync() error
	With(fields ...Field) Logger
}

func String(k, v string) Field {
	return zap.String(k, v)
}

func Duration(k string, d time.Duration) Field {
	return zap.Duration(k, d)
}

func Float64(key string, val float64) Field {
	return zap.Float64(key, val)
}

func Time(key string, val time.Time) Field {
	return zap.Time(key, val)
}

func Int(k string, i int) Field {
	return zap.Int(k, i)
}

func Int64(k string, i int64) Field {
	return zap.Int64(k, i)
}

func Error(v error) Field {
	return zap.Error(v)
}

func Object(key string, val zapcore.ObjectMarshaler) Field {
	return zap.Object(key, val)
}

func NewProductionLogger() (Logger, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}

	return &wrapLogger{logger}, nil
}

func NewDevelopmentLogger() (Logger, error) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}

	return &wrapLogger{logger}, nil
}

func NewNopLogger() (Logger, error) {
	return &wrapLogger{zap.NewNop()}, nil
}

type wrapLogger struct {
	*zap.Logger
}

func (l wrapLogger) With(fields ...Field) Logger {
	return &wrapLogger{l.Logger.With(fields...)}
}

func (l wrapLogger) Panic(ctx context.Context, msg string, fields ...Field) {
	l.Logger.Panic(msg, fields...)
}

func (l wrapLogger) Info(ctx context.Context, msg string, fields ...Field) {
	l.Logger.Info(msg, fields...)

}
func (l wrapLogger) Error(ctx context.Context, msg string, fields ...Field) {
	l.Logger.Error(msg, fields...)
}

func (l wrapLogger) Debug(ctx context.Context, msg string, fields ...Field) {
	l.Logger.Debug(msg, fields...)
}
