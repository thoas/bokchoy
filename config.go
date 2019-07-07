package bokchoy

import "github.com/go-redis/redis"

type RedisClusterConfig redis.ClusterOptions
type RedisClientConfig redis.Options
type RedisSentinelConfig redis.FailoverOptions

// RedisConfig contains all redis configuration: client, sentinel (failover), cluster.
type RedisConfig struct {
	Type     string
	Prefix   string
	Client   RedisClientConfig
	Cluster  RedisClusterConfig
	Sentinel RedisSentinelConfig
}

// QueueConfig contains queue information that should be initialized.
type QueueConfig struct {
	Name string
}

// BrokerConfig contains the broker configuration.
type BrokerConfig struct {
	Type  string
	Redis RedisConfig
}

// Config contains the main configuration to initialize Bokchoy.
type Config struct {
	Queues     []QueueConfig
	Broker     BrokerConfig
	Serializer SerializerConfig
}

// SerializerConfig contains a serializer configuration to store tasks.
type SerializerConfig struct {
	Type string
}
