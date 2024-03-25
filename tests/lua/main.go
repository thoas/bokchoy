package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/davecgh/go-spew/spew"
	"github.com/redis/go-redis/v9"
)

var multihgetall = `local collate = function (key)
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

return cjson.encode(data)
`

var script = `local key = ARGV[1]
local min = ARGV[2]
local max = ARGV[3]
local results = redis.call('ZRANGEBYSCORE', key, min, max)
local length = #results
if length > 0 then
    redis.call('ZREMRANGEBYSCORE', key, min, max)
    return results
else
    return nil
end`

func main() {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	var (
		results map[string]map[string]interface{}
		ctx     context.Context
	)

	sha, err := client.ScriptLoad(ctx, multihgetall).Result()
	if err != nil {
		log.Fatal(err)
	}

	vals, err := client.EvalSha(ctx, sha, []string{"foo"}).Result()
	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal([]byte(vals.(string)), &results)
	if err != nil {
		log.Fatal(err)
	}

	spew.Dump(results)

	sha, err = client.ScriptLoad(ctx, script).Result()
	spew.Dump(sha, err)

	vals, err = client.Eval(ctx, "return {KEYS[1],ARGV[1]}", []string{"key"}, "hello").Result()
	spew.Dump(vals, err)

	vals, err = client.EvalSha(ctx, sha, nil, "myzset", "-inf", "+inf").Result()
	spew.Dump(vals, err)
}
