package bokchoy

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/oklog/ulid"
	"github.com/pkg/errors"
)

func id() string {
	t := time.Unix(1000000, 0)
	entropy := ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)
	return ulid.MustNew(ulid.Now(), entropy).String()
}

func reverseDurations(durations []time.Duration) []time.Duration {
	results := make([]time.Duration, len(durations))

	j := len(durations) - 1
	for i := 0; i < len(durations); i++ {
		results[i] = durations[j]
		j--
	}

	return results
}

func mapDuration(values map[string]interface{}, key string, optional bool) (time.Duration, error) {
	raw, err := mapString(values, key, optional)
	if err != nil {
		return 0, err
	}

	if raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return 0, errors.Wrapf(ErrAttributeError, "cannot parse `%s` to integer", key)
		}

		timeValue := time.Duration(value) * time.Second

		return timeValue, nil
	}

	return 0, nil
}

func mapTime(values map[string]interface{}, key string, optional bool) (time.Time, error) {
	raw, err := mapString(values, key, optional)
	if err != nil {
		return time.Time{}, err
	}

	if raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return time.Time{}, errors.Wrapf(ErrAttributeError, "cannot parse `%s` to integer", key)
		}

		timeValue := time.Unix(value, 0).UTC()

		return timeValue, nil
	}

	return time.Time{}, nil
}

func mapString(values map[string]interface{}, key string, optional bool) (string, error) {
	raw, ok := values[key]
	if !ok && !optional {
		return "", errors.Wrapf(ErrAttributeError, "cannot cast `%s`", key)
	}

	switch raw := raw.(type) {
	case string:
		return raw, nil
	case int, int64:
		return fmt.Sprintf("%d", raw), nil
	}

	return "", nil
}

func mapInt(values map[string]interface{}, key string, optional bool) (int, error) {
	raw, err := mapString(values, key, optional)
	if err != nil {
		return 0, err
	}

	if raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return 0, errors.Wrapf(ErrAttributeError, "cannot parse `%s` to integer", key)
		}

		return int(value), nil
	}

	return 0, nil
}

func mapFloat(values map[string]interface{}, key string, optional bool) (float64, error) {
	raw, err := mapString(values, key, optional)
	if err != nil {
		return 0, err
	}

	if raw != "" {
		value, err := strconv.ParseFloat(raw, 10)
		if err != nil {
			return 0, errors.Wrapf(ErrAttributeError, "cannot parse `%s` to float", key)
		}

		return value, nil
	}

	return 0, nil
}

func unpack(fields map[string]interface{}) []interface{} {
	args := make([]interface{}, len(fields)*2)
	i := 0
	for k, v := range fields {
		args[i] = k
		args[i+1] = v
		i += 2
	}

	return args
}
