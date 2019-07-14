package bokchoy

import "time"

const (
	logo = `
 _           _        _                 
| |__   ___ | | _____| |__   ___  _   _ 
| '_ \ / _ \| |/ / __| '_ \ / _ \| | | |
| |_) | (_) |   < (__| | | | (_) | |_| |
|_.__/ \___/|_|\_\___|_| |_|\___/ \__, |
                                  |___/ `
	defaultTimeout     = 180 * time.Second
	defaultConcurrency = 1
	defaultMaxRetries  = 3
	defaultTTL         = 180 * time.Second

	Version = "v0.1.0"
)

var defaultRetryIntervals = []time.Duration{
	60 * time.Second,
	120 * time.Second,
	180 * time.Second,
}
