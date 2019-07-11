package middleware

import (
	"bytes"
	"context"
	"log"
	"os"
	"time"

	"github.com/thoas/bokchoy"
)

var (
	// LogEntryCtxKey is the context.Context key to store the request log entry.
	LogEntryCtxKey = &contextKey{"LogEntry"}

	// DefaultLogger is called by the Logger middleware handler to log each request.
	// Its made a package-level variable so that it can be reconfigured for custom
	// logging configurations.
	DefaultLogger = RequestLogger(&DefaultLogFormatter{Logger: log.New(os.Stdout, "", log.LstdFlags), NoColor: false})
)

// RequestLogger returns a logger handler using a custom LogFormatter.
func RequestLogger(f LogFormatter) func(next bokchoy.Handler) bokchoy.Handler {
	return func(next bokchoy.Handler) bokchoy.Handler {
		fn := func(r *bokchoy.Request) error {
			entry := f.NewLogEntry(r)

			t1 := time.Now()

			r = bokchoy.WithAfterRequestFunc(r, func() {
				entry.Write(r, time.Since(t1))
			})

			next.Handle(WithLogEntry(r, entry))

			return nil
		}
		return bokchoy.HandlerFunc(fn)
	}
}

// LogFormatter initiates the beginning of a new LogEntry per request.
// See DefaultLogFormatter for an example implementation.
type LogFormatter interface {
	NewLogEntry(r *bokchoy.Request) LogEntry
}

// LogEntry records the final log when a request completes.
// See defaultLogEntry for an example implementation.
type LogEntry interface {
	Write(r *bokchoy.Request, elapsed time.Duration)
	Panic(v interface{}, stack []byte)
}

// GetLogEntry returns the in-context LogEntry for a request.
func GetLogEntry(r *bokchoy.Request) LogEntry {
	entry, _ := r.Context().Value(LogEntryCtxKey).(LogEntry)
	return entry
}

// WithLogEntry sets the in-context LogEntry for a request.
func WithLogEntry(r *bokchoy.Request, entry LogEntry) *bokchoy.Request {
	r = r.WithContext(context.WithValue(r.Context(), LogEntryCtxKey, entry))
	return r
}

// LoggerInterface accepts printing to stdlib logger or compatible logger.
type LoggerInterface interface {
	Print(v ...interface{})
}

// DefaultLogFormatter is a simple logger that implements a LogFormatter.
type DefaultLogFormatter struct {
	Logger  LoggerInterface
	NoColor bool
}

// NewLogEntry creates a new LogEntry for the request.
func (l *DefaultLogFormatter) NewLogEntry(r *bokchoy.Request) LogEntry {
	useColor := !l.NoColor
	entry := &defaultLogEntry{
		DefaultLogFormatter: l,
		request:             r,
		buf:                 &bytes.Buffer{},
		useColor:            useColor,
	}

	reqID := GetReqID(r.Context())
	if reqID != "" {
		cW(entry.buf, useColor, nYellow, "[%s] ", reqID)
	}

	task := r.Task

	cW(entry.buf, useColor, bMagenta, "<Task id=%s name=%s payload=%s>", task.ID, task.Name, task.Payload)
	entry.buf.WriteString(" - ")

	return entry
}

type defaultLogEntry struct {
	*DefaultLogFormatter
	request  *bokchoy.Request
	buf      *bytes.Buffer
	useColor bool
}

func (l *defaultLogEntry) Write(r *bokchoy.Request, elapsed time.Duration) {
	task := r.Task

	switch {
	case task.IsStatusProcessing():
		cW(l.buf, l.useColor, bBlue, "%s", task.StatusDisplay())
	case task.IsStatusSucceeded():
		cW(l.buf, l.useColor, bGreen, "%s", task.StatusDisplay())
	case task.IsStatusCanceled():
		cW(l.buf, l.useColor, bYellow, "%s", task.StatusDisplay())
	case task.IsStatusFailed():
		cW(l.buf, l.useColor, bRed, "%s", task.StatusDisplay())
	}

	l.buf.WriteString(" - ")

	if task.Result == nil {
		cW(l.buf, l.useColor, bBlue, "result: (empty)")
	} else {
		cW(l.buf, l.useColor, bBlue, "result: \"%s\"", task.Result)
	}

	l.buf.WriteString(" in ")
	if elapsed < 500*time.Millisecond {
		cW(l.buf, l.useColor, nGreen, "%s", elapsed)
	} else if elapsed < 5*time.Second {
		cW(l.buf, l.useColor, nYellow, "%s", elapsed)
	} else {
		cW(l.buf, l.useColor, nRed, "%s", elapsed)
	}

	l.Logger.Print(l.buf.String())
}

func (l *defaultLogEntry) Panic(v interface{}, stack []byte) {
	panicEntry := l.NewLogEntry(l.request).(*defaultLogEntry)
	cW(panicEntry.buf, l.useColor, bRed, "panic: %+v", v)
	l.Logger.Print(panicEntry.buf.String())
	l.Logger.Print(string(stack))
}
