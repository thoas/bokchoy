package bokchoy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/thoas/bokchoy/logging"

	"github.com/go-redis/redis"
	"github.com/pkg/errors"
)

type redisClient interface {
	redis.UniversalClient
}

type redisBroker struct {
	cfg     RedisConfig
	clt     redisClient
	prefix  string
	logger  logging.Logger
	scripts map[string]string
}

const (
	// Redis type
	redisTypeSentinel = "sentinel"
	redisTypeCluster  = "cluster"
	redisTypeClient   = "client"
)

var redisScripts = map[string]string{
	"HMSETEXPIRE": `local key = KEYS[1]
local data = ARGV
local ttl = table.remove(data, 1)
local res = redis.call('HMSET', key, unpack(data))
redis.call('EXPIRE', key, ttl)
return res`,
	"ZPOPBYSCORE": `local key = ARGV[1]
local min = ARGV[2]
local max = ARGV[3]
local results = redis.call('ZRANGEBYSCORE', key, min, max)
local length = #results
if length > 0 then
    redis.call('ZREMRANGEBYSCORE', key, min, max)
    return results
else
    return nil
end`,
	"MULTIHGETALL": `local collate = function (key)
  local raw_data = redis.call('HGETALL', key)
  local data = {}

  for idx = 1, #raw_data, 2 do
    data[raw_data[idx]] = raw_data[idx + 1]
  end

  return data;
end

local data = {}

for _, key in ipairs(KEYS) do
  data[key] = collate(key)
end

return cjson.encode(data)`,
}

// newRedisBroker initializes a new redis client.
func newRedisBroker(ctx context.Context, cfg RedisConfig, logger logging.Logger) (*redisBroker, error) {
	var clt redisClient

	switch cfg.Type {
	case redisTypeSentinel:
		clt = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:         cfg.Sentinel.MasterName,
			SentinelAddrs:      cfg.Sentinel.SentinelAddrs,
			Password:           cfg.Sentinel.Password,
			MaxRetries:         cfg.Sentinel.MaxRetries,
			DialTimeout:        cfg.Sentinel.DialTimeout,
			ReadTimeout:        cfg.Sentinel.ReadTimeout,
			WriteTimeout:       cfg.Sentinel.WriteTimeout,
			PoolSize:           cfg.Sentinel.PoolSize,
			PoolTimeout:        cfg.Sentinel.PoolTimeout,
			IdleTimeout:        cfg.Sentinel.IdleTimeout,
			MinIdleConns:       cfg.Sentinel.MinIdleConns,
			MaxConnAge:         cfg.Sentinel.MaxConnAge,
			IdleCheckFrequency: cfg.Sentinel.IdleCheckFrequency,
		})
	case redisTypeCluster:
		clt = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:              cfg.Cluster.Addrs,
			Password:           cfg.Cluster.Password,
			MaxRetries:         cfg.Cluster.MaxRetries,
			DialTimeout:        cfg.Cluster.DialTimeout,
			ReadTimeout:        cfg.Cluster.ReadTimeout,
			WriteTimeout:       cfg.Cluster.WriteTimeout,
			PoolSize:           cfg.Cluster.PoolSize,
			PoolTimeout:        cfg.Cluster.PoolTimeout,
			IdleTimeout:        cfg.Cluster.IdleTimeout,
			MinIdleConns:       cfg.Cluster.MinIdleConns,
			MaxConnAge:         cfg.Cluster.MaxConnAge,
			IdleCheckFrequency: cfg.Cluster.IdleCheckFrequency,
		})
	default:
		clt = redis.NewClient(&redis.Options{
			Addr:               cfg.Client.Addr,
			Password:           cfg.Client.Password,
			DB:                 cfg.Client.DB,
			MaxRetries:         cfg.Client.MaxRetries,
			DialTimeout:        cfg.Client.DialTimeout,
			ReadTimeout:        cfg.Client.ReadTimeout,
			WriteTimeout:       cfg.Client.WriteTimeout,
			PoolSize:           cfg.Client.PoolSize,
			PoolTimeout:        cfg.Client.PoolTimeout,
			IdleTimeout:        cfg.Client.IdleTimeout,
			MinIdleConns:       cfg.Client.MinIdleConns,
			MaxConnAge:         cfg.Client.MaxConnAge,
			IdleCheckFrequency: cfg.Client.IdleCheckFrequency,
		})

	}

	b := &redisBroker{
		clt:    clt,
		prefix: cfg.Prefix,
		logger: logger,
	}

	err := b.initialize(ctx)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (p *redisBroker) initialize(ctx context.Context) error {
	err := p.clt.Ping().Err()
	if err != nil {
		return err
	}

	p.scripts = make(map[string]string)
	for key := range redisScripts {
		sha, err := p.clt.ScriptLoad(redisScripts[key]).Result()
		if err != nil {
			return errors.Wrapf(err, "Unable to load script %s", key)
		}

		p.scripts[key] = sha
	}

	p.logger.Debug(ctx, fmt.Sprintf("connected to redis %s", p.cfg.Type))

	return nil
}

// Ping pings the redis broker to ensure it's well connected.
func (p redisBroker) Ping() error {
	_, err := p.clt.Ping().Result()
	if err != nil {
		return errors.Wrapf(err, "unable to ping redis %s", p.cfg.Type)
	}

	return nil
}

func (p redisBroker) prefixed(keys ...interface{}) string {
	parts := []interface{}{p.prefix}
	parts = append(parts, keys...)

	return fmt.Sprint(parts...)
}

// Consume returns an array of raw data.
func (p *redisBroker) Consume(name string, taskPrefix string, eta time.Time) ([]map[string]interface{}, error) {
	var (
		err      error
		result   []string
		queueKey = p.prefixed(name)
	)

	if eta.IsZero() {
		result, err = p.clt.BRPop(1*time.Second, queueKey).Result()

		if err != nil && err != redis.Nil {
			return nil, errors.Wrapf(err, "unable to BRPOP %s", queueKey)
		}
	} else {
		max := fmt.Sprintf("%d", eta.UTC().Unix())
		vals, err := p.clt.EvalSha(p.scripts["ZPOPBYSCORE"], nil, queueKey, "0", max).Result()
		if err != nil && err != redis.Nil {
			return nil, errors.Wrapf(err, "unable to ZPOPBYSCORE %s", queueKey)
		}

		if vals != nil {
			raw, ok := vals.([]interface{})
			if !ok {
				return nil, errors.Wrapf(err, "unable to cast %v from ZPOPBYSCORE %s", vals, queueKey)
			}

			result = make([]string, len(raw))
			for i := range raw {
				result[i], ok = raw[i].(string)
				if !ok {
					return nil, errors.Wrapf(err, "unable to cast %v from ZPOPBYSCORE %s", raw[i], queueKey)
				}
			}
		}
	}

	if len(result) == 0 {
		return nil, nil
	}

	taskKeys := make([]string, 0, len(result))
	for i := range result {
		if result[i] == name {
			continue
		}

		taskKeys = append(taskKeys, p.prefixed(taskPrefix, result[i]))
	}

	vals, err := p.clt.EvalSha(p.scripts["MULTIHGETALL"], taskKeys).Result()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to HGETALL %s", strings.Join(taskKeys, ", "))
	}

	var values map[string]map[string]interface{}
	err = json.Unmarshal([]byte(vals.(string)), &values)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to unmarshal %s", strings.Join(taskKeys, ", "))
	}

	results := make([]map[string]interface{}, 0, len(result))
	for _, data := range values {
		if len(data) == 0 {
			continue
		}

		results = append(results, data)
	}

	return results, nil
}

// Get returns stored raw data from task key.
func (p *redisBroker) Get(taskKey string) (map[string]interface{}, error) {
	taskKey = p.prefixed(taskKey)

	res, err := p.clt.HGetAll(taskKey).Result()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to HGETALL %s", taskKey)
	}

	results := make(map[string]interface{})
	for k, v := range res {
		results[k] = v
	}

	return results, nil
}

// Count returns number of items from a queue name.
func (p *redisBroker) Count(queueName string) (int, error) {
	queueName = p.prefixed(queueName)

	var res *redis.IntCmd

	value, err := p.clt.Type(queueName).Result()
	if err != nil {
		return 0, errors.Wrapf(err, "unable to TYPE %s", queueName)
	}

	switch value {
	case "zset":
		res = p.clt.ZCount(queueName, "-inf", "+inf")
	case "list":
		res = p.clt.LLen(queueName)
	}

	if res == nil {
		return 0, nil
	}

	if res != nil && res.Err() != nil {
		return 0, errors.Wrapf(res.Err(), "unable to LEN %s", queueName)
	}

	return int(res.Val()), nil
}

// Save synchronizes the stored item in redis.
func (p *redisBroker) Set(taskKey string, data map[string]interface{}, expiration time.Duration) error {
	prefixedTaskKey := p.prefixed(taskKey)

	if int(expiration.Seconds()) == 0 {
		_, err := p.clt.HMSet(prefixedTaskKey, data).Result()
		if err != nil {
			return errors.Wrapf(err, "unable to HMSET %s", prefixedTaskKey)
		}

		return nil
	}

	values := []interface{}{int(expiration.Seconds())}
	values = append(values, unpack(data)...)

	_, err := p.clt.EvalSha(p.scripts["HMSETEXPIRE"], []string{prefixedTaskKey}, values...).Result()
	if err != nil {
		return errors.Wrapf(err, "unable to HMSETEXPIRE %s", prefixedTaskKey)
	}

	return nil
}

// Publish publishes raw data.
// it uses a hash to store the task itself
// pushes the task id to the list or a zset if the task is delayed.
func (p *redisBroker) Publish(queueName string, taskPrefix string,
	taskID string, data map[string]interface{}, eta time.Time) error {
	prefixedTaskKey := p.prefixed(taskPrefix, taskID)

	_, err := p.clt.Pipelined(func(pipe redis.Pipeliner) error {
		pipe.HMSet(prefixedTaskKey, data)

		if eta.IsZero() {
			pipe.RPush(p.prefixed(queueName), taskID)
		} else {
			// if eta is before now, then we should push this
			// taskID in priority
			if eta.Before(time.Now().UTC()) {
				pipe.LPush(p.prefixed(queueName), taskID)
			} else {
				pipe.ZAdd(p.prefixed(queueName), redis.Z{
					Score:  float64(eta.UTC().Unix()),
					Member: taskID,
				})
			}
		}

		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "unable to HMSET %s", prefixedTaskKey)
	}

	return nil
}

// Empty removes the redis key for a queue.
func (p *redisBroker) Empty(name string) error {
	err := p.clt.Del(p.prefixed(name)).Err()
	if err != nil && err != redis.Nil {
		return errors.Wrapf(err, "unable to DEL %s", p.prefixed(name))
	}

	return nil
}

// Flush flushes the entire redis database.
func (p *redisBroker) Flush() error {
	err := p.clt.FlushDB().Err()
	if err != nil {
		return errors.Wrap(err, "unable to FLUSHDB")
	}

	return nil
}

var _ Broker = (*redisBroker)(nil)
