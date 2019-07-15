package bokchoy

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/user"
	"strings"
	"sync"

	"github.com/thoas/bokchoy/logging"
	"github.com/thoas/go-funk"

	"github.com/pkg/errors"
)

// Bokchoy is the main object which stores all configuration, queues
// and broker.
type Bokchoy struct {
	cfg            Config
	wg             *sync.WaitGroup
	defaultOptions *Options
	broker         Broker
	queues         map[string]*Queue
	middlewares    []func(Handler) Handler
	services       []Service

	Serializer Serializer
	Logger     logging.Logger
	Tracer     Tracer
}

// New initializes a new Bokchoy instance.
func New(ctx context.Context, cfg Config, options ...Option) (*Bokchoy, error) {
	opts := newOptions()
	for i := range options {
		options[i](opts)
	}

	var (
		err    error
		tracer Tracer
	)

	logger := logging.NewNopLogger()
	if opts.Logger != nil {
		logger = opts.Logger
	}

	tracer = opts.Tracer
	if tracer == nil {
		tracer = NewLoggerTracer(logger)
	}

	wg := &sync.WaitGroup{}
	bok := &Bokchoy{
		cfg:            cfg,
		Serializer:     newSerializer(cfg.Serializer),
		queues:         make(map[string]*Queue),
		wg:             wg,
		Logger:         logger,
		Tracer:         tracer,
		defaultOptions: opts,
		services:       opts.Services,
	}

	if opts.Serializer != nil {
		bok.Serializer = opts.Serializer
	}

	bok.broker = newBroker(ctx, cfg.Broker,
		logger.With(logging.String("component", "broker")))

	if opts.Initialize {
		bok.Logger.Debug(ctx, fmt.Sprintf("Connecting to %s...", bok.broker))

		err = bok.broker.Initialize(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "unable to initialize broker")
		}

		bok.Logger.Debug(ctx, fmt.Sprintf("Connected to %s", bok.broker))
	}

	for i := range cfg.Queues {
		bok.Queue(cfg.Queues[i].Name)
	}

	return bok, nil
}

// Use append a new middleware to the system.
func (b *Bokchoy) Use(sub ...func(Handler) Handler) *Bokchoy {
	b.middlewares = append(b.middlewares, sub...)

	return b
}

// Empty empties initialized queues.
func (b *Bokchoy) Empty(ctx context.Context) error {
	for i := range b.queues {
		err := b.queues[i].Empty(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// Flush flushes data of the entire system.
func (b *Bokchoy) Flush() error {
	return b.broker.Flush()
}

// Queue gets or creates a new queue.
func (b *Bokchoy) Queue(name string) *Queue {
	queue, ok := b.queues[name]
	if !ok {
		queue = &Queue{
			name:           name,
			broker:         b.broker,
			serializer:     b.Serializer,
			logger:         b.Logger.With(logging.String("component", "queue")),
			tracer:         b.Tracer,
			wg:             b.wg,
			defaultOptions: b.defaultOptions,
			middlewares:    b.middlewares,
		}

		b.queues[name] = queue
	}

	return queue
}

// Stop stops all queues and consumers.
func (b *Bokchoy) Stop(ctx context.Context) {
	fields := []logging.Field{
		logging.String("queues", strings.Join(b.QueueNames(), ", ")),
	}

	b.Logger.Debug(ctx, "Stopping queues...", fields...)
	for i := range b.queues {
		b.queues[i].stop(ctx)
	}
	b.Logger.Debug(ctx, "Queues stopped", fields...)

	if len(b.services) == 0 {
		return
	}

	fields = []logging.Field{
		logging.String("services", strings.Join(b.ServiceNames(), ", ")),
	}

	b.Logger.Debug(ctx, "Stopping services...", fields...)
	for i := range b.services {
		b.services[i].Stop(ctx)

		b.wg.Done()
	}
	b.Logger.Debug(ctx, "Services stopped", fields...)
}

// QueueNames returns the managed queue names.
func (b *Bokchoy) QueueNames() []string {
	names := make([]string, 0, len(b.queues))

	for k := range b.queues {
		names = append(names, k)
	}

	return names
}

// ServiceNames returns the managed server names.
func (b *Bokchoy) ServiceNames() []string {
	names := make([]string, 0, len(b.services))

	for i := range b.services {
		names = append(names, fmt.Sprintf("%s", b.services[i]))
	}

	return names
}

func (b *Bokchoy) displayOutput(ctx context.Context, queueNames []string) {
	buf := &bytes.Buffer{}
	ColorWrite(buf, true, ColorBrightGreen, "%s\n", logo)

	user, err := user.Current()
	if err == nil {
		hostname, err := os.Hostname()
		if err == nil {
			ColorWrite(buf, true, ColorBrightBlue, "%s@%s %v\n", user.Username, hostname, Version)
			ColorWrite(buf, true, ColorBrightBlue, "- uid: %s\n", user.Uid)
			ColorWrite(buf, true, ColorBrightBlue, "- gid: %s\n\n", user.Gid)
		}
	}

	ColorWrite(buf, true, ColorBrightBlue, "[config]\n")
	ColorWrite(buf, true, ColorBrightBlue, fmt.Sprintf("- concurrency: %d\n", b.defaultOptions.Concurrency))
	ColorWrite(buf, true, ColorBrightBlue, fmt.Sprintf("- serializer: %s\n", b.Serializer))
	ColorWrite(buf, true, ColorBrightBlue, fmt.Sprintf("- max retries: %d\n", b.defaultOptions.MaxRetries))
	ColorWrite(buf, true, ColorBrightBlue, fmt.Sprintf("- retry intervals: %s\n", b.defaultOptions.RetryIntervalsDisplay()))
	ColorWrite(buf, true, ColorBrightBlue, fmt.Sprintf("- ttl: %s\n", b.defaultOptions.TTL))
	ColorWrite(buf, true, ColorBrightBlue, fmt.Sprintf("- countdown: %s\n", b.defaultOptions.Countdown))
	ColorWrite(buf, true, ColorBrightBlue, fmt.Sprintf("- timeout: %s\n", b.defaultOptions.Timeout))
	ColorWrite(buf, true, ColorBrightBlue, fmt.Sprintf("- tracer: %s\n", b.Tracer))
	ColorWrite(buf, true, ColorBrightBlue, fmt.Sprintf("- broker: %s\n", b.broker))
	ColorWrite(buf, true, ColorBrightBlue, "\n[queues]\n")

	for i := range queueNames {
		ColorWrite(buf, true, ColorBrightBlue, fmt.Sprintf("- %s", queueNames[i]))
	}

	if len(b.services) > 0 {
		ColorWrite(buf, true, ColorBrightBlue, "\n\n[services]\n")

		for i := range b.services {
			ColorWrite(buf, true, ColorBrightBlue, fmt.Sprintf("- %s", b.services[i]))
		}
	}

	log.Print(buf)
}

// Run runs the system and block the current goroutine.
func (b *Bokchoy) Run(ctx context.Context, options ...Option) error {
	opts := newOptions()
	for i := range options {
		options[i](opts)
	}

	if len(opts.Services) > 0 {
		b.services = opts.Services
	}

	err := b.broker.Ping()
	if err != nil {
		return err
	}

	queueNames := b.QueueNames()
	if len(opts.Queues) > 0 {
		queueNames = funk.FilterString(queueNames, func(queueName string) bool {
			return funk.InStrings(opts.Queues, queueName)
		})
	}

	if len(queueNames) == 0 {
		b.Logger.Debug(ctx, "No queue to run...")

		return ErrNoQueueToRun
	}

	fields := []logging.Field{
		logging.String("queues", strings.Join(queueNames, ", ")),
	}

	b.Logger.Debug(ctx, "Starting queues...", fields...)

	for i := range b.queues {
		if !funk.InStrings(queueNames, b.queues[i].Name()) {
			continue
		}

		b.queues[i].start(ctx)
	}

	b.Logger.Debug(ctx, "Queues started", fields...)

	fields = []logging.Field{
		logging.String("services", strings.Join(b.ServiceNames(), ", ")),
	}

	if len(b.services) > 0 {
		b.Logger.Debug(ctx, "Starting services...", fields...)

		for i := range b.services {
			b.wg.Add(1)

			go func(service Service) {
				err := service.Start(ctx)
				if err != nil {
					b.Logger.Error(ctx, fmt.Sprintf("Receive error when starting %s", service), logging.Error(err))
				}
			}(b.services[i])
		}

		b.Logger.Debug(ctx, "Services started", fields...)
	}

	if !b.defaultOptions.DisableOutput {
		b.displayOutput(ctx, queueNames)
	}

	b.wg.Wait()

	return nil
}

// Publish publishes a new payload to a queue.
func (b *Bokchoy) Publish(ctx context.Context, queueName string, payload interface{}, options ...Option) (*Task, error) {
	return b.Queue(queueName).Publish(ctx, payload, options...)
}

// Handle registers a new handler to consume tasks for a queue.
func (b *Bokchoy) Handle(queueName string, sub Handler, options ...Option) {
	b.HandleFunc(queueName, sub.Handle)
}

// HandleFunc registers a new handler function to consume tasks for a queue.
func (b *Bokchoy) HandleFunc(queueName string, f HandlerFunc, options ...Option) {
	b.Queue(queueName).HandleFunc(f, options...)
}
