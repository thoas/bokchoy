package bokchoy

import "time"

const (
	defaultTimeout     = 180 * time.Second
	defaultConcurrency = 1
	defaultMaxRetries  = 3
	defaultTTL         = 180 * time.Second
)

var defaultRetryIntervals = []time.Duration{
	60 * time.Second,
	120 * time.Second,
	180 * time.Second,
}
