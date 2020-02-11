package bokchoy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/thoas/bokchoy/logging"

	"github.com/go-redis/redis"
	"github.com/pkg/errors"
)

type redisClient interface {
	redis.UniversalClient
}

// RedisBroker is the redis broker.
type RedisBroker struct {
	ClientType string
	Client     redisClient
	Prefix     string
	Logger     logging.Logger
	scripts    map[string]string
	mu         *sync.Mutex
	queues     map[string]struct{}
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
func newRedisBroker(cfg RedisConfig, logger logging.Logger) *RedisBroker {
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

	return NewRedisBroker(clt, cfg.Type, cfg.Prefix, logger)
}

// NewRedisBroker initializes a new redis broker instance.
func NewRedisBroker(clt redisClient, clientType string, prefix string, logger logging.Logger) *RedisBroker {
	return &RedisBroker{
		ClientType: clientType,
		Client:     clt,
		Prefix:     prefix,
		Logger:     logger,
		queues:     make(map[string]struct{}),
		mu:         &sync.Mutex{},
	}
}

func (p RedisBroker) String() string {
	return fmt.Sprintf("redis (%s)", p.ClientType)
}

// Initialize initializes the redis broker.
func (p *RedisBroker) Initialize(ctx context.Context) error {
	err := p.Client.Ping().Err()
	if err != nil {
		return err
	}

	p.scripts = make(map[string]string)
	for key := range redisScripts {
		sha, err := p.Client.ScriptLoad(redisScripts[key]).Result()
		if err != nil {
			return errors.Wrapf(err, "Unable to load script %s", key)
		}

		p.scripts[key] = sha
	}

	return nil
}

// Ping pings the redis broker to ensure it's well connected.
func (p RedisBroker) Ping() error {
	_, err := p.Client.Ping().Result()
	if err != nil {
		return errors.Wrapf(err, "unable to ping redis %s", p.ClientType)
	}

	return nil
}

func (p RedisBroker) prefixed(keys ...interface{}) string {
	parts := []interface{}{p.Prefix}
	parts = append(parts, keys...)

	return fmt.Sprint(parts...)
}

func (p *RedisBroker) consumeDelayed(ctx context.Context, name string, duration time.Duration) {
	p.mu.Lock()

	delayName := fmt.Sprint(name, ":delay")
	_, ok := p.queues[delayName]
	if !ok {
		go func() {
			ticker := time.NewTicker(duration)

			for range ticker.C {
				max := time.Now().UTC()

				results, err := p.consume(ctx, delayName, name, max)
				if err != nil {
					p.Logger.Error(ctx, "Received error when retrieving delayed payloads",
						logging.Error(err))
				}

				if len(results) == 0 {
					continue
				}

				_, err = p.Client.TxPipelined(func(pipe redis.Pipeliner) error {
					for i := range results {
						taskID, ok := results[i]["id"].(string)
						if !ok {
							continue
						}

						err := p.publish(pipe, name, taskID, results[i], time.Time{})
						if err != nil {
							return err
						}
					}

					// To avoid data loss, we only remove the range when results are processed
					_, err = pipe.ZRemRangeByScore(delayName, "0", fmt.Sprintf("%d", max.Unix())).Result()
					if err != nil {
						return err
					}

					return nil
				})
			}
		}()

		p.queues[delayName] = struct{}{}
	}

	p.mu.Unlock()

}

func (p *RedisBroker) consume(ctx context.Context, name string, taskPrefix string, eta time.Time) ([]map[string]interface{}, error) {
	var (
		err      error
		result   []string
		queueKey = p.prefixed(name)
	)

	if eta.IsZero() {
		p.consumeDelayed(ctx, name, 1*time.Second)

		result, err = p.Client.BRPop(1*time.Second, queueKey).Result()

		if err != nil && err != redis.Nil {
			return nil, errors.Wrapf(err, "unable to BRPOP %s", queueKey)
		}
	} else {
		max := fmt.Sprintf("%d", eta.UTC().Unix())
		results := p.Client.ZRangeByScore(queueKey, redis.ZRangeBy{
			Min: "0",
			Max: max,
		})

		if results.Err() != nil && results.Err() != redis.Nil {
			return nil, errors.Wrapf(err, "unable to ZRANGEBYSCORE %s", queueKey)
		}

		result = results.Val()
	}

	if len(result) == 0 {
		return nil, nil
	}

	taskKeys := make([]string, 0, len(result))
	for i := range result {
		if result[i] == name {
			continue
		}

		taskKeys = append(taskKeys, p.prefixed(taskPrefix, ":", result[i]))
	}

	values, err := p.payloadsFromKeys(taskKeys)
	if err != nil {
		return nil, err
	}

	results := make([]map[string]interface{}, 0, len(taskKeys))
	for _, data := range values {
		if len(data) == 0 {
			continue
		}

		results = append(results, data)
	}

	return results, nil
}

// Consume returns an array of raw data.
func (p *RedisBroker) Consume(ctx context.Context, name string, eta time.Time) ([]map[string]interface{}, error) {
	return p.consume(ctx, name, name, eta)

}

func (p *RedisBroker) payloadsFromKeys(taskKeys []string) (map[string]map[string]interface{}, error) {
	vals, err := p.Client.EvalSha(p.scripts["MULTIHGETALL"], taskKeys).Result()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to MULTIHGETALL %s", strings.Join(taskKeys, ", "))
	}

	var values map[string]map[string]interface{}
	err = json.Unmarshal([]byte(vals.(string)), &values)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to unmarshal %s", strings.Join(taskKeys, ", "))
	}

	return values, nil
}

// Get returns stored raw data from task key.
func (p *RedisBroker) Get(taskKey string) (map[string]interface{}, error) {
	taskKey = p.prefixed(taskKey)

	res, err := p.Client.HGetAll(taskKey).Result()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to HGETALL %s", taskKey)
	}

	results := make(map[string]interface{})
	for k, v := range res {
		results[k] = v
	}

	return results, nil
}

// Delete deletes raw data in broker based on key.
func (p *RedisBroker) Delete(name string, taskID string) error {
	return p.delete(p.Client, name, taskID)
}

func (p *RedisBroker) delete(client redis.Cmdable, name string, taskID string) error {
	var (
		prefixedTaskKey = p.prefixed(name, ":", taskID)
	)

	_, err := client.Del(prefixedTaskKey).Result()
	if err != nil {
		return errors.Wrapf(err, "unable to DEL %s", prefixedTaskKey)
	}

	return nil
}

func (p *RedisBroker) List(name string) ([]map[string]interface{}, error) {
	taskIDs, err := p.Client.LRange(name, 0, -1).Result()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to LRANGE %s", name)
	}

	taskKeys := make([]string, 0, len(taskIDs))
	for i := range taskIDs {
		taskKeys = append(taskKeys, p.prefixed(name, ":", taskIDs[i]))
	}

	payloads, err := p.payloadsFromKeys(taskKeys)
	if err != nil {
		return nil, err
	}

	results := make([]map[string]interface{}, 0, len(taskKeys))
	for _, data := range payloads {
		if len(data) == 0 {
			continue
		}

		results = append(results, data)
	}

	return results, nil
}

// Count returns number of items from a queue name.
func (p *RedisBroker) Count(queueName string) (BrokerStats, error) {
	var (
		stats = BrokerStats{}
		err   error
	)

	queueName = p.prefixed(queueName)
	direct, err := p.Client.LLen(queueName).Result()
	if err != nil && err != redis.Nil {
		return stats, err
	}

	stats.Direct = int(direct)

	delayed, err := p.Client.ZCount(fmt.Sprint(queueName, ":delay"), "-inf", "+inf").Result()
	if err != nil && err != redis.Nil {
		return stats, err
	}

	stats.Delayed = int(delayed)

	stats.Total = stats.Direct + stats.Delayed

	return stats, nil
}

// Save synchronizes the stored item in redis.
func (p *RedisBroker) Set(taskKey string, data map[string]interface{}, expiration time.Duration) error {
	prefixedTaskKey := p.prefixed(taskKey)

	if int(expiration.Seconds()) == 0 {
		_, err := p.Client.HMSet(prefixedTaskKey, data).Result()
		if err != nil {
			return errors.Wrapf(err, "unable to HMSET %s", prefixedTaskKey)
		}

		return nil
	}

	values := []interface{}{int(expiration.Seconds())}
	values = append(values, unpack(data)...)

	_, err := p.Client.EvalSha(p.scripts["HMSETEXPIRE"], []string{prefixedTaskKey}, values...).Result()
	if err != nil {
		return errors.Wrapf(err, "unable to HMSETEXPIRE %s", prefixedTaskKey)
	}

	return nil
}

// Publish publishes raw data.
// it uses a hash to store the task itself
// pushes the task id to the list or a zset if the task is delayed.
func (p *RedisBroker) Publish(queueName string,
	taskID string, data map[string]interface{}, eta time.Time) error {

	_, err := p.Client.Pipelined(func(pipe redis.Pipeliner) error {
		return p.publish(pipe, queueName, taskID, data, eta)
	})
	if err != nil {
		return err
	}

	return nil
}

func (p *RedisBroker) publish(client redis.Cmdable, queueName string,
	taskID string, data map[string]interface{}, eta time.Time) error {

	var (
		prefixedTaskKey = p.prefixed(queueName, ":", taskID)
		err             error
	)

	err = client.HMSet(prefixedTaskKey, data).Err()
	if err == nil {
		if eta.IsZero() {
			err = client.RPush(p.prefixed(queueName), taskID).Err()
		} else {
			// if eta is before now, then we should push this
			// taskID in priority
			if eta.Before(time.Now().UTC()) {
				err = client.LPush(p.prefixed(queueName), taskID).Err()
			} else {
				err = client.ZAdd(p.prefixed(fmt.Sprint(queueName, ":delay")), redis.Z{
					Score:  float64(eta.UTC().Unix()),
					Member: taskID,
				}).Err()
			}
		}
	}
	if err != nil {
		return errors.Wrapf(err, "unable to HMSET %s", taskID)
	}

	return nil
}

// Empty removes the redis key for a queue.
func (p *RedisBroker) Empty(name string) error {
	err := p.Client.Del(p.prefixed(name)).Err()
	if err != nil && err != redis.Nil {
		return errors.Wrapf(err, "unable to DEL %s", p.prefixed(name))
	}

	return nil
}

// Flush flushes the entire redis database.
func (p *RedisBroker) Flush() error {
	err := p.Client.FlushDB().Err()
	if err != nil {
		return errors.Wrap(err, "unable to FLUSHDB")
	}

	return nil
}

var _ Broker = (*RedisBroker)(nil)
