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

			ctx := bokchoy.WithContextAfterRequestFunc(r.Context(), func() {
				entry.Write(r, time.Since(t1))
			})

			r = r.WithContext(ctx)

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
		bokchoy.ColorWrite(entry.buf, useColor, bokchoy.ColorYellow, "[%s] ", reqID)
	}

	task := r.Task

	bokchoy.ColorWrite(entry.buf, useColor, bokchoy.ColorBrightMagenta, "<Task id=%s name=%s payload=%v>", task.ID, task.Name, task.Payload)
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
		bokchoy.ColorWrite(l.buf, l.useColor, bokchoy.ColorBrightBlue, "%s", task.StatusDisplay())
	case task.IsStatusSucceeded():
		bokchoy.ColorWrite(l.buf, l.useColor, bokchoy.ColorBrightGreen, "%s", task.StatusDisplay())
	case task.IsStatusCanceled():
		bokchoy.ColorWrite(l.buf, l.useColor, bokchoy.ColorBrightYellow, "%s", task.StatusDisplay())
	case task.IsStatusFailed():
		bokchoy.ColorWrite(l.buf, l.useColor, bokchoy.ColorBrightRed, "%s", task.StatusDisplay())
	}

	l.buf.WriteString(" - ")

	if task.Result == nil {
		bokchoy.ColorWrite(l.buf, l.useColor, bokchoy.ColorBrightBlue, "result: (empty)")
	} else {
		bokchoy.ColorWrite(l.buf, l.useColor, bokchoy.ColorBrightBlue, "result: \"%s\"", task.Result)
	}

	l.buf.WriteString(" in ")
	if elapsed < 500*time.Millisecond {
		bokchoy.ColorWrite(l.buf, l.useColor, bokchoy.ColorGreen, "%s", elapsed)
	} else if elapsed < 5*time.Second {
		bokchoy.ColorWrite(l.buf, l.useColor, bokchoy.ColorYellow, "%s", elapsed)
	} else {
		bokchoy.ColorWrite(l.buf, l.useColor, bokchoy.ColorRed, "%s", elapsed)
	}

	l.Logger.Print(l.buf.String())
}

func (l *defaultLogEntry) Panic(v interface{}, stack []byte) {
	panicEntry := l.NewLogEntry(l.request).(*defaultLogEntry)
	bokchoy.ColorWrite(panicEntry.buf, l.useColor, bokchoy.ColorRed, "panic: %+v", v)
	l.Logger.Print(panicEntry.buf.String())
	l.Logger.Print(string(stack))
}
