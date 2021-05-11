package bokchoy

import (
	"context"
	"time"

	"github.com/thoas/bokchoy/logging"
)

// Broker is the common interface to define a Broker.
type Broker interface {
	// Initialize initializes the broker.
	Initialize(context.Context) error

	// Ping pings the broker to ensure it's well connected.
	Ping(context.Context) error

	// Get returns raw data stored in broker.
	Get(context.Context, string) (map[string]interface{}, error)

	// Delete deletes raw data in broker based on key.
	Delete(context.Context, string, string) error

	// List returns raw data stored in broker.
	List(context.Context, string) ([]map[string]interface{}, error)

	// Empty empties a queue.
	Empty(context.Context, string) error

	// Flush flushes the entire broker.
	Flush(context.Context) error

	// Count returns number of items from a queue name.
	Count(context.Context, string) (BrokerStats, error)

	// Save synchronizes the stored item.
	Set(context.Context, string, map[string]interface{}, time.Duration) error

	// Publish publishes raw data.
	Publish(context.Context, string, string, map[string]interface{}, time.Time) error

	// Consume returns an array of raw data.
	Consume(context.Context, string, time.Time) ([]map[string]interface{}, error)
}

// BrokerStats is the statistics returned by a Queue.
type BrokerStats struct {
	Total   int
	Direct  int
	Delayed int
}

// newBroker initializes a new Broker instance.
func newBroker(cfg BrokerConfig, logger logging.Logger) Broker {
	var (
		broker Broker
	)

	switch cfg.Type {
	default:
		broker = newRedisBroker(cfg.Redis, logger)
	}

	return broker
}
