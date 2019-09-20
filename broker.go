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
	Ping() error

	// Get returns raw data stored in broker.
	Get(string) (map[string]interface{}, error)

	// Delete deletes raw data in broker based on key.
	Delete(string, string) error

	// List returns raw data stored in broker.
	List(string) ([]map[string]interface{}, error)

	// Empty empties a queue.
	Empty(string) error

	// Flush flushes the entire broker.
	Flush() error

	// Count returns number of items from a queue name.
	Count(string) (BrokerStats, error)

	// Save synchronizes the stored item.
	Set(string, map[string]interface{}, time.Duration) error

	// Publish publishes raw data.
	Publish(string, string, map[string]interface{}, time.Time) error

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
func newBroker(ctx context.Context, cfg BrokerConfig, logger logging.Logger) Broker {
	var (
		broker Broker
	)

	switch cfg.Type {
	default:
		broker = newRedisBroker(ctx, cfg.Redis, logger)
	}

	return broker
}
