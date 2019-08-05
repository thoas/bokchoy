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
	clientType string
	cfg        RedisConfig
	clt        redisClient
	prefix     string
	logger     logging.Logger
	scripts    map[string]string
}

const (
	// Redis type
	redisTypeSentinel = "sentinel"
	redisTypeCluster  = "cluster"
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
func newRedisBroker(ctx context.Context, cfg BrokerConfig, logger logging.Logger) *redisBroker {
	var clt redisClient
	if cfg.RedisClient != nil {
		clt = cfg.RedisClient
	} else {
		switch cfg.Redis.Type {
		case redisTypeSentinel:
			clt = redis.NewFailoverClient(&redis.FailoverOptions{
				MasterName:         cfg.Redis.Sentinel.MasterName,
				SentinelAddrs:      cfg.Redis.Sentinel.SentinelAddrs,
				Password:           cfg.Redis.Sentinel.Password,
				MaxRetries:         cfg.Redis.Sentinel.MaxRetries,
				DialTimeout:        cfg.Redis.Sentinel.DialTimeout,
				ReadTimeout:        cfg.Redis.Sentinel.ReadTimeout,
				WriteTimeout:       cfg.Redis.Sentinel.WriteTimeout,
				PoolSize:           cfg.Redis.Sentinel.PoolSize,
				PoolTimeout:        cfg.Redis.Sentinel.PoolTimeout,
				IdleTimeout:        cfg.Redis.Sentinel.IdleTimeout,
				MinIdleConns:       cfg.Redis.Sentinel.MinIdleConns,
				MaxConnAge:         cfg.Redis.Sentinel.MaxConnAge,
				IdleCheckFrequency: cfg.Redis.Sentinel.IdleCheckFrequency,
			})
		case redisTypeCluster:
			clt = redis.NewClusterClient(&redis.ClusterOptions{
				Addrs:              cfg.Redis.Cluster.Addrs,
				Password:           cfg.Redis.Cluster.Password,
				MaxRetries:         cfg.Redis.Cluster.MaxRetries,
				DialTimeout:        cfg.Redis.Cluster.DialTimeout,
				ReadTimeout:        cfg.Redis.Cluster.ReadTimeout,
				WriteTimeout:       cfg.Redis.Cluster.WriteTimeout,
				PoolSize:           cfg.Redis.Cluster.PoolSize,
				PoolTimeout:        cfg.Redis.Cluster.PoolTimeout,
				IdleTimeout:        cfg.Redis.Cluster.IdleTimeout,
				MinIdleConns:       cfg.Redis.Cluster.MinIdleConns,
				MaxConnAge:         cfg.Redis.Cluster.MaxConnAge,
				IdleCheckFrequency: cfg.Redis.Cluster.IdleCheckFrequency,
			})
		default:
			clt = redis.NewClient(&redis.Options{
				Addr:               cfg.Redis.Client.Addr,
				Password:           cfg.Redis.Client.Password,
				DB:                 cfg.Redis.Client.DB,
				MaxRetries:         cfg.Redis.Client.MaxRetries,
				DialTimeout:        cfg.Redis.Client.DialTimeout,
				ReadTimeout:        cfg.Redis.Client.ReadTimeout,
				WriteTimeout:       cfg.Redis.Client.WriteTimeout,
				PoolSize:           cfg.Redis.Client.PoolSize,
				PoolTimeout:        cfg.Redis.Client.PoolTimeout,
				IdleTimeout:        cfg.Redis.Client.IdleTimeout,
				MinIdleConns:       cfg.Redis.Client.MinIdleConns,
				MaxConnAge:         cfg.Redis.Client.MaxConnAge,
				IdleCheckFrequency: cfg.Redis.Client.IdleCheckFrequency,
			})

		}
	}

	return &redisBroker{
		clientType: cfg.Redis.Type,
		clt:        clt,
		prefix:     cfg.Redis.Prefix,
		logger:     logger,
	}
}

func (p redisBroker) String() string {
	return fmt.Sprintf("redis (%s)", p.clientType)
}

// Initialize initializes the redis broker.
func (p *redisBroker) Initialize(ctx context.Context) error {
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

	if res.Err() != nil {
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
