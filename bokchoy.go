package bokchoy

import (
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
	servers        []Server

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
		servers:        opts.Servers,
	}

	if opts.Serializer != nil {
		bok.Serializer = opts.Serializer
	}

	if opts.Broker != nil {
		bok.broker = opts.Broker
	} else {
		bok.broker = newBroker(ctx, cfg.Broker,
			logger.With(logging.String("component", "broker")))
	}

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

	if len(b.servers) == 0 {
		return
	}

	fields = []logging.Field{
		logging.String("servers", strings.Join(b.ServerNames(), ", ")),
	}

	b.Logger.Debug(ctx, "Stopping servers...", fields...)
	for i := range b.servers {
		b.servers[i].Stop(ctx)

		b.wg.Done()
	}
	b.Logger.Debug(ctx, "Servers stopped", fields...)
}

// QueueNames returns the managed queue names.
func (b *Bokchoy) QueueNames() []string {
	names := make([]string, 0, len(b.queues))

	for k := range b.queues {
		names = append(names, k)
	}

	return names
}

// ServerNames returns the managed server names.
func (b *Bokchoy) ServerNames() []string {
	names := make([]string, 0, len(b.servers))

	for i := range b.servers {
		names = append(names, fmt.Sprintf("%s", b.servers[i]))
	}

	return names
}

func (b *Bokchoy) displayOutput(ctx context.Context, queueNames []string) {
	buf := NewColorWriter(ColorBrightGreen)
	buf.Write("%s\n", logo)
	buf = buf.WithColor(ColorBrightBlue)

	user, err := user.Current()
	if err == nil {
		hostname, err := os.Hostname()
		if err == nil {
			buf.Write("%s@%s %v\n", user.Username, hostname, Version)
			buf.Write("- uid: %s\n", user.Uid)
			buf.Write("- gid: %s\n\n", user.Gid)
		}
	}

	buf.Write("[config]\n")
	buf.Write(fmt.Sprintf("- concurrency: %d\n", b.defaultOptions.Concurrency))
	buf.Write(fmt.Sprintf("- serializer: %s\n", b.Serializer))
	buf.Write(fmt.Sprintf("- max retries: %d\n", b.defaultOptions.MaxRetries))
	buf.Write(fmt.Sprintf("- retry intervals: %s\n", b.defaultOptions.RetryIntervalsDisplay()))
	buf.Write(fmt.Sprintf("- ttl: %s\n", b.defaultOptions.TTL))
	buf.Write(fmt.Sprintf("- countdown: %s\n", b.defaultOptions.Countdown))
	buf.Write(fmt.Sprintf("- timeout: %s\n", b.defaultOptions.Timeout))
	buf.Write(fmt.Sprintf("- tracer: %s\n", b.Tracer))
	buf.Write(fmt.Sprintf("- broker: %s\n", b.broker))
	buf.Write("\n[queues]\n")

	for i := range queueNames {
		buf.Write(fmt.Sprintf("- %s\n", queueNames[i]))
	}

	if len(b.servers) > 0 {
		buf.Write("\n[servers]\n")

		for i := range b.servers {
			buf.Write(fmt.Sprintf("- %s", b.servers[i]))
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

	if len(opts.Servers) > 0 {
		b.servers = opts.Servers
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
		logging.String("servers", strings.Join(b.ServerNames(), ", ")),
	}

	if len(b.servers) > 0 {
		b.Logger.Debug(ctx, "Starting servers...", fields...)

		for i := range b.servers {
			b.wg.Add(1)

			go func(server Server) {
				err := server.Start(ctx)
				if err != nil {
					b.Logger.Error(ctx, fmt.Sprintf("Receive error when starting %s", server), logging.Error(err))
				}
			}(b.servers[i])
		}

		b.Logger.Debug(ctx, "Servers started", fields...)
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
	b.HandleFunc(queueName, sub.Handle, options...)
}

// HandleFunc registers a new handler function to consume tasks for a queue.
func (b *Bokchoy) HandleFunc(queueName string, f HandlerFunc, options ...Option) {
	b.Queue(queueName).HandleFunc(f, options...)
}
