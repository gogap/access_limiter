package access_limiter

import (
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
)

type RedisConfig struct {
	IdleTimeout time.Duration
	MaxIdle     int
	MaxActive   int

	Address  string
	Password string
	Db       int

	Prefix string
}

type RedisCounterStorage struct {
	redisConfig RedisConfig

	pool *redis.Pool
}

func NewRedisCounterStorage(config RedisConfig) CounterStorage {
	storage := new(RedisCounterStorage)

	storage.redisConfig = config

	storage.initalPool()

	return storage
}

func (p *RedisCounterStorage) initalPool() {
	p.pool = &redis.Pool{
		MaxIdle:     p.redisConfig.MaxIdle,
		IdleTimeout: p.redisConfig.IdleTimeout,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp",
				p.redisConfig.Address,
				redis.DialDatabase(p.redisConfig.Db),
				redis.DialPassword(p.redisConfig.Password))

			if err != nil {
				return nil, err
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
		Wait: true,
	}
}

func (p *RedisCounterStorage) Increase(counterName string, count int64, dimensions ...string) (err error) {
	conn := p.pool.Get()
	defer conn.Close()

	dimKey := ""
	if dimensions != nil {
		dimKey = strings.Join(dimensions, ":")
	}

	_, err = conn.Do("HINCRBY", p.redisConfig.Prefix+":_counter_:"+counterName, dimKey, count)

	return
}

func (p *RedisCounterStorage) Delete(counterName string, dimensions ...string) (err error) {
	conn := p.pool.Get()
	defer conn.Close()

	dimKey := ""
	if dimensions != nil {
		dimKey = strings.Join(dimensions, ":")
	}

	if dimKey != "" {
		_, err = conn.Do("HDEL", p.redisConfig.Prefix+":_counter_:"+counterName, dimKey)
	} else {
		_, err = conn.Do("DEL", p.redisConfig.Prefix+":_counter_:"+counterName)
	}

	return
}

func (p *RedisCounterStorage) SetValue(counterName string, value int64, dimensions ...string) (err error) {
	conn := p.pool.Get()
	defer conn.Close()

	dimKey := ""
	if dimensions != nil {
		dimKey = strings.Join(dimensions, ":")
	}

	_, err = conn.Do("HSET", p.redisConfig.Prefix+":_counter_:"+counterName, dimKey, value)
	return
}

func (p *RedisCounterStorage) GetValue(counterName string, dimensions ...string) (dimVal int64, exist bool) {
	conn := p.pool.Get()
	defer conn.Close()

	dimKey := ""
	if dimensions != nil {
		dimKey = strings.Join(dimensions, ":")
	}

	v, err := conn.Do("HGET", p.redisConfig.Prefix+":_counter_:"+counterName, dimKey)

	if dimVal, err = redis.Int64(v, err); err == nil {
		exist = true
	}

	return
}

func (p *RedisCounterStorage) GetSumValue(counterName string, dimensionsGroup [][]string) (sumDimVal int64, err error) {
	if dimensionsGroup == nil {
		return
	}

	conn := p.pool.Get()
	defer conn.Close()

	dimKeys := []interface{}{p.redisConfig.Prefix + ":_counter_:" + counterName}

	for _, dim := range dimensionsGroup {
		dimKey := ""
		if dim != nil {
			dimKey = strings.Join(dim, ":")
		}

		dimKeys = append(dimKeys, dimKey)
	}

	vals, err := conn.Do("HMGET", dimKeys...)

	var intVals []int
	if intVals, err = redis.Ints(vals, err); err != nil {
		return
	}

	sumV := 0
	for _, v := range intVals {
		sumV += v
	}

	sumDimVal = int64(sumV)

	return
}

func (p *RedisCounterStorage) GetOption(counterName, key string) (opts []CounterOption, exist bool) {
	conn := p.pool.Get()
	defer conn.Close()

	redisKey := p.redisConfig.Prefix + ":_config_:" + counterName + ":" + key

	vals, err := conn.Do("HGETALL", redisKey)

	if kvVals, e := redis.StringMap(vals, err); e != nil {
		exist = false
		return
	} else {
		for k, v := range kvVals {
			opts = append(opts, CounterOption{OptionName(k), v})
		}
		exist = true
	}

	return
}

func (p *RedisCounterStorage) SetOptions(counterName, key string, opts ...CounterOption) (err error) {
	if opts == nil || len(opts) == 0 {
		return
	}

	conn := p.pool.Get()
	defer conn.Close()

	redisKey := p.redisConfig.Prefix + ":_config_:" + counterName + ":" + key

	args := []interface{}{redisKey}
	for _, opt := range opts {
		args = append(args, opt.Name, opt.Value)
	}

	_, err = conn.Do("HMSET", args...)

	return
}
