package bokchoy

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/thoas/bokchoy/logging"

	"github.com/pkg/errors"
)

// QueueStats is the statistics returned by a Queue.
type QueueStats struct {
	Total   int
	Direct  int
	Delayed int
}

// Queue contains consumers to enqueue.
type Queue struct {
	broker Broker

	name           string
	serializer     Serializer
	logger         logging.Logger
	tracer         Tracer
	consumers      []*consumer
	defaultOptions *Options
	wg             *sync.WaitGroup
	middlewares    []func(Subscriber) Subscriber
	onFailure      []Subscriber
	onSuccess      []Subscriber
	onComplete     []Subscriber
	onStart        []Subscriber
}

// Use appends a new subscriber middleware to the queue.
func (q *Queue) Use(sub ...func(Subscriber) Subscriber) *Queue {
	q.middlewares = append(q.middlewares, sub...)

	return q
}

// OnStart registers a new subscriber to be executed when a task is started.
func (q *Queue) OnStart(sub Subscriber) *Queue {
	q.OnStartFunc(sub.Consume)

	return q
}

// OnStartFunc registers a new subscriber function to be executed when a task is started.
func (q *Queue) OnStartFunc(f SubscriberFunc) *Queue {
	q.onStart = append(q.onStart, f)

	return q
}

// OnComplete registers a new subscriber to be executed when a task is completed.
func (q *Queue) OnComplete(sub Subscriber) *Queue {
	q.OnCompleteFunc(sub.Consume)

	return q
}

// OnCompleteFunc registers a new subscriber function to be executed when a task is completed.
func (q *Queue) OnCompleteFunc(f SubscriberFunc) *Queue {
	q.onComplete = append(q.onComplete, f)

	return q
}

// OnFailure registers a new subscriber to be executed when a task is failed.
func (q *Queue) OnFailure(sub Subscriber) *Queue {
	return q.OnFailureFunc(sub.Consume)
}

// OnFailureFunc registers a new subscriber function to be executed when a task is failed.
func (q *Queue) OnFailureFunc(f SubscriberFunc) *Queue {
	q.onFailure = append(q.onFailure, f)

	return q
}

// OnSuccess registers a new subscriber to be executed when a task is succeeded.
func (q *Queue) OnSuccess(sub Subscriber) *Queue {
	return q.OnSuccessFunc(sub.Consume)
}

// OnSuccessFunc registers a new subscriber function to be executed when a task is succeeded.
func (q *Queue) OnSuccessFunc(f SubscriberFunc) *Queue {
	q.onSuccess = append(q.onSuccess, f)

	return q
}

// Name returns the queue name.
func (q Queue) Name() string {
	return q.name
}

// DelayName returns the delayed queue name.
func (q Queue) DelayName() string {
	return fmt.Sprintf("%s:delay", q.name)
}

// Subscribe registers a new subscriber to consume tasks.
func (q *Queue) Subscribe(sub Subscriber, options ...Option) *Queue {
	return q.SubscribeFunc(sub.Consume)
}

// SubscribeFunc registers a new subscriber function to consume tasks.
func (q *Queue) SubscribeFunc(f SubscriberFunc, options ...Option) *Queue {
	opts := q.defaultOptions

	if len(options) > 0 {
		opts = newOptions()
		for i := range options {
			options[i](opts)
		}
	}

	for i := 0; i < opts.Concurrency; i++ {
		consumer := &consumer{
			name:       q.name,
			handler:    f,
			queue:      q,
			serializer: q.serializer,
			logger: q.logger.With(logging.String("component",
				fmt.Sprintf("consumer:%s#%d", q.name, i+1))),
			tracer:      q.tracer,
			wg:          q.wg,
			mu:          &sync.Mutex{},
			middlewares: q.middlewares,
		}
		q.consumers = append(q.consumers, consumer)
	}

	return q
}

// consumeDelayedTasks consumes delayed tasks.
func (q *Queue) consumeDelayedTasks(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Second)

		for range ticker.C {
			tasks, err := q.ConsumeDelayed(ctx)
			if err != nil {
				q.tracer.Log(ctx, "Received error when retrieving delayed tasks", err)
			}

			for i := range tasks {
				err := q.PublishTask(ctx, tasks[i])
				if err != nil {
					q.logger.Debug(ctx, "Unable to re-publish delayed task",
						logging.Error(err),
						logging.Object("queue", q),
						logging.Object("task", tasks[i]))
				}

				q.logger.Debug(ctx, "Delayed task re-published",
					logging.Object("queue", q),
					logging.Object("task", tasks[i]))
			}
		}
	}()
}

// start starts consumers.
func (q *Queue) start(ctx context.Context) {
	q.logger.Debug(ctx, "Starting consumers...",
		logging.Object("queue", q))

	for i := range q.consumers {
		q.consumers[i].start(ctx)
	}

	q.consumeDelayedTasks(ctx)

	q.logger.Debug(ctx, "Consumers started",
		logging.Object("queue", q))
}

// Empty empties queue.
func (q *Queue) Empty(ctx context.Context) error {
	queueNames := []string{q.name, q.DelayName()}

	q.logger.Debug(ctx, "Emptying queue...",
		logging.Object("queue", q))

	for i := range queueNames {
		err := q.broker.Empty(queueNames[i])
		if err != nil {
			return errors.Wrapf(err, "unable to empty queue %s", queueNames[i])
		}
	}

	q.logger.Debug(ctx, "Queue emptied",
		logging.Object("queue", q))

	return nil
}

// MarshalLogObject returns the log representation for the queue.
func (q Queue) MarshalLogObject(enc logging.ObjectEncoder) error {
	enc.AddString("name", q.name)
	enc.AddInt("consumers_count", len(q.consumers))

	return nil
}

// stop stops consumers.
func (q *Queue) stop(ctx context.Context) {
	q.logger.Debug(ctx, "Stopping consumers...",
		logging.Object("queue", q))

	for i := range q.consumers {
		q.consumers[i].stop(ctx)
	}

	q.logger.Debug(ctx, "Consumers stopped",
		logging.Object("queue", q))
}

// TaskKey returns the task key prefixed by the queue name.
func (q Queue) TaskKey(taskID string) string {
	return fmt.Sprintf("%s:%s", q.name, taskID)
}

// Cancel cancels a task using its ID.
func (q *Queue) Cancel(ctx context.Context, taskID string) (*Task, error) {
	task, err := q.Get(ctx, taskID)
	if err != nil {
		return nil, err
	}

	task.MarkAsCanceled()

	err = q.Save(ctx, task)
	if err != nil {
		return nil, err
	}

	return task, nil
}

// Get returns a task instance from the broker with its id.
func (q *Queue) Get(ctx context.Context, taskID string) (*Task, error) {
	start := time.Now()

	results, err := q.broker.Get(q.TaskKey(taskID))
	if err != nil {
		return nil, err
	}

	task, err := taskFromPayload(results, q.serializer)
	if err != nil {
		return nil, err
	}

	q.logger.Debug(ctx, "Task retrieved",
		logging.Object("queue", q),
		logging.Duration("duration", time.Since(start)),
		logging.Object("task", task))

	return task, err
}

// Count returns statistics from queue:
// * direct: number of waiting tasks
// * delayed: number of waiting delayed tasks
// * total: number of total tasks
func (q *Queue) Count(ctx context.Context) (QueueStats, error) {
	var err error

	stats := QueueStats{}
	stats.Direct, err = q.broker.Count(q.name)
	if err != nil {
		return stats, err
	}

	stats.Delayed, err = q.broker.Count(q.DelayName())
	if err != nil {
		return stats, err
	}

	stats.Total = stats.Direct + stats.Delayed

	return stats, nil
}

// ConsumeDelayed returns an array of delayed tasks.
func (q *Queue) ConsumeDelayed(ctx context.Context) ([]*Task, error) {
	return q.consume(ctx, q.DelayName(), q.name, time.Now().UTC())
}

// Consume returns an array of tasks.
func (q *Queue) Consume(ctx context.Context) ([]*Task, error) {
	return q.consume(ctx, q.name, q.name, time.Time{})
}

func (q *Queue) consume(ctx context.Context, name string, prefix string, eta time.Time) ([]*Task, error) {
	results, err := q.broker.Consume(name, fmt.Sprintf("%s:", prefix), eta)
	if err != nil {
		return nil, err
	}

	tasks := make([]*Task, 0, len(results))

	for i := range results {
		task, err := taskFromPayload(results[i], q.serializer)
		if err != nil {
			q.tracer.Log(ctx, "Receive error when casting payload to Task", err)
			continue
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

func (q *Queue) consumer() *consumer {
	rand.Seed(time.Now().Unix())

	n := rand.Int() % len(q.consumers)

	return q.consumers[n]
}

func (q *Queue) fireEvents(r *Request) error {
	task := r.Task

	if task.IsStatusProcessing() {
		for i := range q.onStart {
			err := q.onStart[i].Consume(r)
			if err != nil {
				return errors.Wrapf(err, "unable to handle onStart %s", r)
			}
		}
	}

	if task.IsStatusSucceeded() {
		for i := range q.onSuccess {
			err := q.onSuccess[i].Consume(r)
			if err != nil {
				return errors.Wrapf(err, "unable to handle onSuccess %s", r)
			}
		}
	}

	if task.IsStatusFailed() || task.IsStatusCanceled() {
		for i := range q.onFailure {
			err := q.onFailure[i].Consume(r)
			if err != nil {
				return errors.Wrapf(err, "unable to handle onFailure %s", r)
			}
		}
	}

	if task.Finished() {
		for i := range q.onComplete {
			err := q.onComplete[i].Consume(r)
			if err != nil {
				return errors.Wrapf(err, "unable to handle onComplete %s", task)
			}
		}

	}

	return nil
}

// HandleRequest handles a request synchronously with a consumer.
func (q *Queue) HandleRequest(ctx context.Context, r *Request) error {
	consumer := q.consumer()

	return consumer.Consume(r)
}

// Save saves a task to the queue.
func (q *Queue) Save(ctx context.Context, task *Task) error {
	var err error

	start := time.Now()

	data, err := task.Serialize(q.serializer)
	if err != nil {
		return err
	}

	if task.Finished() {
		err = q.broker.Set(task.Key(), data, task.TTL)
	} else {
		err = q.broker.Set(task.Key(), data, 0)
	}

	if err != nil {
		return errors.Wrapf(err, "unable to save %s", task)
	}

	q.logger.Debug(ctx, "Task saved",
		logging.Object("queue", q),
		logging.Duration("duration", time.Since(start)),
		logging.Object("task", task))

	return nil
}

// NewTask returns a new task instance from payload and options.
func (q *Queue) NewTask(payload interface{}, options ...Option) *Task {
	opts := q.defaultOptions

	if len(options) > 0 {
		opts = newOptions()
		for i := range options {
			options[i](opts)
		}
	}

	task := NewTask(q.name, payload)
	task.MaxRetries = opts.MaxRetries
	task.TTL = opts.TTL
	task.Timeout = opts.Timeout
	task.RetryIntervals = opts.RetryIntervals

	var eta time.Time

	if int(opts.Countdown.Seconds()) > 0 {
		eta = time.Now().Add(opts.Countdown).UTC()
	}
	task.ETA = eta

	return task
}

// Publish publishes a new payload to the queue.
func (q *Queue) Publish(ctx context.Context, payload interface{}, options ...Option) (*Task, error) {
	task := q.NewTask(payload, options...)

	err := q.PublishTask(ctx, task)
	if err != nil {
		return nil, err
	}

	return task, nil
}

// PublishTask publishes a new task to the queue.
func (q *Queue) PublishTask(ctx context.Context, task *Task) error {
	data, err := task.Serialize(q.serializer)
	if err != nil {
		return err
	}

	queueName := q.name

	// if eta is after now then it should be a delayed task
	if !task.ETA.IsZero() && task.ETA.After(time.Now().UTC()) {
		queueName = q.DelayName()
	}

	taskPrefix := fmt.Sprintf("%s:", q.name)

	start := time.Now()

	err = q.broker.Publish(queueName, taskPrefix, task.ID, data, task.ETA)
	if err != nil {
		return errors.Wrapf(err, "unable to publish %s", task)
	}

	q.logger.Debug(ctx, "Task published",
		logging.Object("queue", q),
		logging.Duration("duration", time.Since(start)),
		logging.Object("task", task))

	return nil
}
