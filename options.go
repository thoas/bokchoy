package bokchoy

import (
	"strings"
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
	Countdown      *time.Duration
	Timeout        time.Duration
	RetryIntervals []time.Duration
	Serializer     Serializer
	Initialize     bool
	Queues         []string
	DisableOutput  bool
	Services       []Service
}

// RetryIntervalsDisplay returns a string representation of the retry intervals.
func (o Options) RetryIntervalsDisplay() string {
	intervals := make([]string, len(o.RetryIntervals))
	for i := range o.RetryIntervals {
		intervals[i] = o.RetryIntervals[i].String()
	}

	return strings.Join(intervals, ", ")
}

// newOptions returns default options.
func newOptions() *Options {
	opts := &Options{}

	options := []Option{
		WithConcurrency(defaultConcurrency),
		WithMaxRetries(defaultMaxRetries),
		WithTTL(defaultTTL),
		WithTimeout(defaultTimeout),
		WithRetryIntervals(defaultRetryIntervals),
		WithInitialize(true),
	}

	for i := range options {
		options[i](opts)
	}

	return opts
}

// Option is an option unit.
type Option func(opts *Options)

// WithDisableOutput defines if the output (logo, queues information)
// should be disabled.
func WithDisableOutput(disableOutput bool) Option {
	return func(opts *Options) {
		opts.DisableOutput = disableOutput
	}
}

// WithServices registers new services to be run.
func WithServices(services []Service) Option {
	return func(opts *Options) {
		opts.Services = services
	}
}

// WithQueues allows to override queues to run.
func WithQueues(queues []string) Option {
	return func(opts *Options) {
		opts.Queues = queues
	}
}

// WithSerializer defines the Serializer.
func WithSerializer(serializer Serializer) Option {
	return func(opts *Options) {
		opts.Serializer = serializer
	}
}

// WithInitialize defines if the broker needs to be initialized.
func WithInitialize(initialize bool) Option {
	return func(opts *Options) {
		opts.Initialize = initialize
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
		opts.Countdown = &countdown
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
