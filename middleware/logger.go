package middleware

import (
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

			return next.Handle(WithLogEntry(r, entry))
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
	entry := &defaultLogEntry{
		DefaultLogFormatter: l,
		request:             r,
		buf:                 bokchoy.NewColorWriter(nil),
	}

	reqID := GetReqID(r.Context())
	if reqID != "" {
		entry.buf = entry.buf.WithColor(bokchoy.ColorYellow)
		entry.buf.Write("[%s] ", reqID)
	}

	task := r.Task

	entry.buf = entry.buf.WithColor(bokchoy.ColorBrightMagenta)
	entry.buf.Write("<Task id=%s name=%s payload=%v>", task.ID, task.Name, task.Payload)
	entry.buf = entry.buf.WithColor(bokchoy.ColorWhite)
	entry.buf.WriteString(" - ")

	return entry
}

type defaultLogEntry struct {
	*DefaultLogFormatter
	request *bokchoy.Request
	buf     *bokchoy.ColorWriter
}

func (l *defaultLogEntry) Write(r *bokchoy.Request, elapsed time.Duration) {
	task := r.Task

	switch {
	case task.IsStatusProcessing():
		l.buf = l.buf.WithColor(bokchoy.ColorBrightBlue)
		l.buf.Write("%s", task.StatusDisplay())
	case task.IsStatusSucceeded():
		l.buf = l.buf.WithColor(bokchoy.ColorBrightGreen)
		l.buf.Write("%s", task.StatusDisplay())
	case task.IsStatusCanceled():
		l.buf = l.buf.WithColor(bokchoy.ColorBrightYellow)
		l.buf.Write("%s", task.StatusDisplay())
	case task.IsStatusFailed():
		l.buf = l.buf.WithColor(bokchoy.ColorBrightRed)
		l.buf.Write("%s", task.StatusDisplay())
	}

	l.buf.WriteString(" - ")

	l.buf = l.buf.WithColor(bokchoy.ColorBrightBlue)

	if task.Result == nil {
		l.buf.Write("result: (empty)")
	} else {
		l.buf.Write("result: \"%s\"", task.Result)
	}

	l.buf = l.buf.WithColor(bokchoy.ColorWhite)

	l.buf.WriteString(" in ")
	if elapsed < 500*time.Millisecond {
		l.buf = l.buf.WithColor(bokchoy.ColorGreen)
		l.buf.Write("%s", elapsed)
	} else if elapsed < 5*time.Second {
		l.buf = l.buf.WithColor(bokchoy.ColorYellow)
		l.buf.Write("%s", elapsed)
	} else {
		l.buf = l.buf.WithColor(bokchoy.ColorRed)
		l.buf.Write("%s", elapsed)
	}

	l.Logger.Print(l.buf.String())
}

func (l *defaultLogEntry) Panic(v interface{}, stack []byte) {
	panicEntry := l.NewLogEntry(l.request).(*defaultLogEntry)
	l.buf = l.buf.WithColor(bokchoy.ColorRed)
	l.buf.Write("panic: %+v", v)
	l.Logger.Print(panicEntry.buf.String())
	l.Logger.Print(string(stack))
}
