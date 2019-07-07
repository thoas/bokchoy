package bokchoy

import (
	"time"

	"github.com/thoas/bokchoy/logging"
)

// Options is the bokchoy options.
type Options struct {
	Tracer         Tracer
	Logger         logging.Logger
	Concurrency    int
	MaxRetries     int
	TTL            time.Duration
	Countdown      time.Duration
	Timeout        time.Duration
	RetryIntervals []time.Duration
	Serializer     Serializer
}

func newOptions() *Options {
	return &Options{
		Concurrency:    defaultConcurrency,
		MaxRetries:     defaultMaxRetries,
		TTL:            defaultTTL,
		Timeout:        defaultTimeout,
		RetryIntervals: defaultRetryIntervals,
	}
}

// Option is an option unit.
type Option func(opts *Options)

// WithSerializer defines the Serializer.
func WithSerializer(serializer Serializer) Option {
	return func(opts *Options) {
		opts.Serializer = serializer
	}
}

// WithTracer defines the Tracer.
func WithTracer(tracer Tracer) Option {
	return func(opts *Options) {
		opts.Tracer = tracer
	}
}

// WithLogger defines the Logger.
func WithLogger(logger logging.Logger) Option {
	return func(opts *Options) {
		opts.Logger = logger
	}
}

// WithTimeout defines the timeout used to execute a task.
func WithTimeout(timeout time.Duration) Option {
	return func(opts *Options) {
		opts.Timeout = timeout
	}
}

// WithCountdown defines the countdown to launch a delayed task.
func WithCountdown(countdown time.Duration) Option {
	return func(opts *Options) {
		opts.Countdown = countdown
	}
}

// WithConcurrency defines the number of concurrent consumers.
func WithConcurrency(concurrency int) Option {
	return func(opts *Options) {
		opts.Concurrency = concurrency
	}
}

// WithMaxRetries defines the number of maximum retries for a failed task.
func WithMaxRetries(maxRetries int) Option {
	return func(opts *Options) {
		opts.MaxRetries = maxRetries
	}
}

// WithRetryIntervals defines the retry intervals for a failed task.
func WithRetryIntervals(retryIntervals []time.Duration) Option {
	return func(opts *Options) {
		opts.RetryIntervals = retryIntervals
	}
}

// WithTTL defines the duration to keep the task in the broker.
func WithTTL(ttl time.Duration) Option {
	return func(opts *Options) {
		opts.TTL = ttl
	}
}
