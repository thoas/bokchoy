package bokchoy

import (
	"context"
	"time"

	"github.com/thoas/bokchoy/logging"
)

type Broker interface {
	// Ping pings the redis broker to ensure it's well connected.
	Ping() error

	// Get returns raw data stored in broker.
	Get(string) (map[string]interface{}, error)

	// Empty empties a queue.
	Empty(string) error

	// Flush flushes the entire broker.
	Flush() error

	// Count returns number of items from a queue name.
	Count(string) (int, error)

	// Save synchronizes the stored item.
	Set(string, map[string]interface{}, time.Duration) error

	// Publish publishes raw data.
	Publish(string, string, string, map[string]interface{}, time.Time) error

	// Consume returns an array of raw data.
	Consume(string, string, time.Time) ([]map[string]interface{}, error)
}

// newBroker initializes a new Broker instance.
func newBroker(ctx context.Context, cfg BrokerConfig, logger logging.Logger) (Broker, error) {
	var (
		broker Broker
		err    error
	)

	switch cfg.Type {
	default:
		broker, err = newRedisBroker(ctx, cfg.Redis, logger)
		if err != nil {
			return nil, err
		}
	}

	return broker, err
}
