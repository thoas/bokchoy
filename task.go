package bokchoy

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/thoas/bokchoy/logging"
)

const (
	// Task statuses
	taskStatusWaiting int = iota
	taskStatusProcessing
	taskStatusSucceeded
	taskStatusFailed
	taskStatusCanceled
)

// Task is the model stored in a Queue.
type Task struct {
	ID             string
	Name           string
	PublishedAt    time.Time
	StartedAt      time.Time
	ProcessedAt    time.Time
	Status         int
	OldStatus      int
	MaxRetries     int
	Payload        interface{}
	Result         interface{}
	Error          interface{}
	ExecTime       float64
	TTL            time.Duration
	Timeout        time.Duration
	ETA            time.Time
	RetryIntervals []time.Duration
}

// NewTask initializes a new Task.
func NewTask(name string, payload interface{}, options ...Option) *Task {
	opts := newOptions()
	for i := range options {
		options[i](opts)
	}

	t := &Task{
		ID:          id(),
		Name:        name,
		Payload:     payload,
		Status:      taskStatusWaiting,
		PublishedAt: time.Now().UTC(),
	}

	t.MaxRetries = opts.MaxRetries
	t.TTL = opts.TTL

	return t
}

// TaskFromPayload returns a Task instance from raw data.
func TaskFromPayload(data map[string]interface{}, serializer Serializer) (*Task, error) {
	var ok bool
	var err error

	t := &Task{}
	t.ID, err = mapString(data, "id", false)
	if err != nil {
		return nil, err
	}

	t.Name, err = mapString(data, "name", false)
	if err != nil {
		return nil, err
	}

	t.Status, err = mapInt(data, "status", false)
	if err != nil {
		return nil, err
	}

	t.OldStatus = t.Status

	t.PublishedAt, err = mapTime(data, "published_at", false)
	if err != nil {
		return nil, err
	}

	t.ProcessedAt, err = mapTime(data, "processed_at", true)
	if err != nil {
		return nil, err
	}

	t.StartedAt, err = mapTime(data, "started_at", true)
	if err != nil {
		return nil, err
	}

	t.ETA, err = mapTime(data, "eta", true)
	if err != nil {
		return nil, err
	}

	t.Timeout, err = mapDuration(data, "timeout", true)
	if err != nil {
		return nil, err
	}

	payload, err := mapString(data, "payload", false)
	if err != nil {
		return nil, err
	}

	t.MaxRetries, err = mapInt(data, "max_retries", false)
	if err != nil {
		return nil, err
	}

	t.TTL, err = mapDuration(data, "ttl", false)
	if err != nil {
		return nil, err
	}

	t.ExecTime, err = mapFloat(data, "exec_time", true)
	if err != nil {
		return nil, err
	}

	rawRetryIntervals, err := mapString(data, "retry_intervals", true)
	if err != nil {
		return nil, err
	}

	if rawRetryIntervals != "" {
		strRetryIntervals := strings.Split(rawRetryIntervals, ",")
		t.RetryIntervals = make([]time.Duration, len(strRetryIntervals))

		for i := range strRetryIntervals {
			value, err := strconv.ParseInt(strRetryIntervals[i], 10, 64)
			if err != nil {
				return nil, errors.Wrapf(ErrAttributeError, "cannot parse %s retry interval to integer", strRetryIntervals[i])
			}

			t.RetryIntervals[i] = time.Duration(value) * time.Second
		}
	}

	err = serializer.Loads([]byte(payload), &t.Payload)
	if err != nil {
		return nil, errors.Wrapf(ErrAttributeError, "cannot unserialize `payload`")
	}

	rawError, ok := data["error"].(string)
	if ok {
		err = serializer.Loads([]byte(rawError), &t.Error)

		if err != nil {
			return nil, errors.Wrapf(ErrAttributeError, "cannot unserialize `error`")
		}
	}

	return t, nil
}

// ETADisplay returns the string representation of the ETA.
func (t Task) ETADisplay() string {
	if t.ETA.IsZero() {
		return "0s"
	}

	return t.ETA.Sub(time.Now().UTC()).String()
}

// RetryETA returns the next ETA.
func (t Task) RetryETA() time.Time {
	if t.MaxRetries >= len(t.RetryIntervals) {
		return t.ETA
	}

	if len(t.RetryIntervals) > 0 {
		intervals := reverseDurations(t.RetryIntervals)

		if len(intervals) > t.MaxRetries {
			return time.Now().UTC().Add(intervals[t.MaxRetries])
		}

		return time.Now().UTC().Add(intervals[0])
	}

	return time.Time{}
}

// MarshalLogObject returns the log representation for the task.
func (t Task) MarshalLogObject(enc logging.ObjectEncoder) error {
	enc.AddString("id", t.ID)
	enc.AddString("name", t.Name)
	enc.AddString("status", t.StatusDisplay())
	enc.AddString("payload", fmt.Sprintf("%v", t.Payload))
	enc.AddInt("max_retries", t.MaxRetries)
	enc.AddDuration("ttl", t.TTL)
	enc.AddDuration("timeout", t.Timeout)
	enc.AddTime("published_at", t.PublishedAt)

	if !t.StartedAt.IsZero() {
		enc.AddTime("started_at", t.StartedAt)
	}

	if !t.ProcessedAt.IsZero() {
		enc.AddTime("processed_at", t.ProcessedAt)
		enc.AddDuration("duration", t.ProcessedAt.Sub(t.StartedAt))
	}

	if !t.ETA.IsZero() {
		enc.AddTime("eta", t.ETA)
	}

	if t.ExecTime != 0 {
		enc.AddFloat64("exec_time", t.ExecTime)
	}

	if len(t.RetryIntervals) > 0 {
		enc.AddString("retry_intervals", t.RetryIntervalsDisplay())
	}

	return nil
}

// RetryIntervalsDisplay returns the string representation of the retry intervals.
func (t Task) RetryIntervalsDisplay() string {
	intervals := make([]string, len(t.RetryIntervals))
	for i := range t.RetryIntervals {
		intervals[i] = fmt.Sprintf("%d", int(t.RetryIntervals[i].Seconds()))
	}

	return strings.Join(intervals, ",")
}

// String returns the string representation of Task.
func (t Task) String() string {
	return fmt.Sprintf(
		"<Task name=%s id=%s, status=%s, published_at=%s>",
		t.Name, t.ID, t.StatusDisplay(), t.PublishedAt.String(),
	)
}

// StatusDisplay returns the status in human representation.
func (t Task) StatusDisplay() string {
	switch t.Status {
	case taskStatusSucceeded:
		return "succeeded"
	case taskStatusProcessing:
		return "processing"
	case taskStatusFailed:
		return "failed"
	case taskStatusCanceled:
		return "canceled"
	}

	return "waiting"
}

// Serialize serializes a Task to raw data.
func (t Task) Serialize(serializer Serializer) (map[string]interface{}, error) {
	var err error

	data := map[string]interface{}{
		"id":           t.ID,
		"name":         t.Name,
		"status":       t.Status,
		"published_at": t.PublishedAt.Unix(),
		"max_retries":  t.MaxRetries,
		"ttl":          int(t.TTL.Seconds()),
		"timeout":      int(t.Timeout.Seconds()),
	}

	if !t.ETA.IsZero() {
		data["eta"] = t.ETA.Unix()
	}

	if !t.ProcessedAt.IsZero() {
		data["processed_at"] = t.ProcessedAt.Unix()
	}

	if !t.StartedAt.IsZero() {
		data["started_at"] = t.StartedAt.Unix()
	}

	if t.Payload != nil {
		payload, err := serializer.Dumps(t.Payload)
		if err != nil {
			return nil, err
		}

		data["payload"] = string(payload)
	}

	if t.Result != nil {
		result, err := serializer.Dumps(t.Result)
		if err != nil {
			return nil, err
		}

		data["result"] = string(result)
	}

	if t.Error != nil {
		rawErr, ok := t.Error.(error)
		if ok {
			data["error"], err = serializer.Dumps(rawErr.Error())
			if err != nil {
				return nil, err
			}
		}
	}

	if t.ExecTime != 0 {
		data["exec_time"] = t.ExecTime
	}

	if len(t.RetryIntervals) > 0 {
		data["retry_intervals"] = t.RetryIntervalsDisplay()
	}

	return data, err
}

// Key returns the task key.
func (t Task) Key() string {
	return fmt.Sprintf("%s:%s", t.Name, t.ID)
}

// MarkAsProcessing marks a task as processing.
func (t *Task) MarkAsProcessing() {
	t.StartedAt = time.Now().UTC()
	t.Status = taskStatusProcessing
}

// Finished returns if a task is finished or not.
func (t *Task) Finished() bool {
	if t.OldStatus == taskStatusSucceeded {
		return true
	}

	if (t.OldStatus == taskStatusFailed || t.Status == taskStatusFailed) && t.MaxRetries == 0 {
		return true
	}

	if t.Status == taskStatusSucceeded {
		return true
	}

	return false
}

// IsStatusWaiting returns if the task status is waiting.
func (t *Task) IsStatusWaiting() bool {
	return t.Status == taskStatusWaiting
}

// IsStatusSucceeded returns if the task status is succeeded.
func (t *Task) IsStatusSucceeded() bool {
	return t.Status == taskStatusSucceeded
}

// IsStatusProcessing returns if the task status is processing.
func (t *Task) IsStatusProcessing() bool {
	return t.Status == taskStatusProcessing
}

// IsStatusFailed returns if the task status is failed.
func (t *Task) IsStatusFailed() bool {
	return t.Status == taskStatusFailed
}

// IsStatusCanceled returns if the task status is canceled.
func (t *Task) IsStatusCanceled() bool {
	return t.Status == taskStatusCanceled
}

// MarkAsSucceeded marks a task as succeeded.
func (t *Task) MarkAsSucceeded() {
	t.ProcessedAt = time.Now().UTC()
	t.Status = taskStatusSucceeded
	t.ExecTime = t.ProcessedAt.Sub(t.StartedAt).Seconds()
}

// MarkAsFailed marks a task as failed.
func (t *Task) MarkAsFailed(err error) {
	t.ProcessedAt = time.Now().UTC()
	t.Status = taskStatusFailed
	if err != nil {
		t.Error = err
	}
	t.ExecTime = t.ProcessedAt.Sub(t.StartedAt).Seconds()
}

// MarkAsCanceled marks a task as canceled.
func (t *Task) MarkAsCanceled() {
	t.ProcessedAt = time.Now().UTC()
	t.Status = taskStatusCanceled
}
