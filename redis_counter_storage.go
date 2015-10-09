package access_limiter

import (
	"encoding/json"
	"github.com/gogap/errors"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
)

const (
	_COUNTER_ = ":_counter_:"
	_CONFIG_  = ":_counter_:_config_:"
)

type RedisConfig struct {
	IdleTimeout time.Duration
	MaxIdle     int
	MaxActive   int

	Address  string
	Password string
	Db       int

	Prefix string

	Transaction     bool
	ConsistentRetry int
}

type RedisCounterStorage struct {
	redisConfig RedisConfig

	pool *redis.Pool
}

func NewRedisCounterStorage(config RedisConfig) CounterStorage {
	storage := new(RedisCounterStorage)

	if config.IdleTimeout <= 0 {
		config.IdleTimeout = time.Second
	}

	if config.ConsistentRetry < 0 {
		config.ConsistentRetry = 3
	}

	if config.MaxIdle <= 0 {
		config.MaxIdle = 1
	}

	if config.MaxActive <= 0 {
		config.MaxActive = 1
	}

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

func (p *RedisCounterStorage) Increase(counterName string, count, max int64, dimensions ...string) (err error) {
	conn := p.pool.Get()
	defer conn.Close()

	dimKey := ""
	if dimensions != nil {
		dimKey = strings.Join(dimensions, ":")
	}

	key := p.redisConfig.Prefix + _COUNTER_ + counterName + ":" + dimKey

	if max > 0 && p.redisConfig.Transaction {
		for i := 0; i < p.redisConfig.ConsistentRetry+1; i++ {

			if _, err = conn.Do("WATCH", key); err != nil {
				continue
			}

			var reply interface{}
			if reply, err = conn.Do("GET", key); err != nil {
				continue
			}

			intV, _ := redis.Int64(reply, err)

			if max > 0 && intV+count > max {
				conn.Do("UNWATCH")
				err = ERR_QUOTA_REACHED_UPPER_LIMIT.New(errors.Params{"counter": counterName, "dimensions": strings.Join(dimensions, ":")})
				return
			}

			if err = conn.Send("MULTI"); err != nil {
				continue
			}
			if err = conn.Send("SET", key, intV+count); err != nil {
				continue
			}

			if _, err = conn.Do("EXEC"); err != nil {
				continue
			} else {
				break
			}
		}
	} else {
		_, err = conn.Do("INCRBY", key, count)
	}

	return
}

func (p *RedisCounterStorage) Delete(counterName string, dimensions ...string) (err error) {
	conn := p.pool.Get()
	defer conn.Close()

	dimKey := ""
	if dimensions != nil {
		dimKey = strings.Join(dimensions, ":")
	}

	_, err = conn.Do("DEL", p.redisConfig.Prefix+_COUNTER_+counterName+":"+dimKey)

	return
}

func (p *RedisCounterStorage) SetValue(counterName string, value int64, dimensions ...string) (err error) {
	conn := p.pool.Get()
	defer conn.Close()

	dimKey := ""
	if dimensions != nil {
		dimKey = strings.Join(dimensions, ":")
	}

	_, err = conn.Do("SET", p.redisConfig.Prefix+_COUNTER_+counterName+":"+dimKey, value)
	return
}

func (p *RedisCounterStorage) GetValue(counterName string, dimensions ...string) (dimVal int64, exist bool) {
	conn := p.pool.Get()
	defer conn.Close()

	dimKey := ""
	if dimensions != nil {
		dimKey = strings.Join(dimensions, ":")
	}

	v, err := conn.Do("GET", p.redisConfig.Prefix+_COUNTER_+counterName+":"+dimKey)

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

	keyPrefix := p.redisConfig.Prefix + _COUNTER_ + counterName

	keys := []interface{}{}

	for _, dim := range dimensionsGroup {
		dimKey := ""
		if dim != nil {
			dimKey = strings.Join(dim, ":")
		}

		keys = append(keys, keyPrefix+":"+dimKey)
	}

	vals, err := conn.Do("MGET", keys...)

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

func (p *RedisCounterStorage) GetOptions(counterName, key string) (opts CounterOptions, exist bool) {
	conn := p.pool.Get()
	defer conn.Close()

	redisKey := p.redisConfig.Prefix + _CONFIG_ + counterName + ":" + key

	vals, err := conn.Do("GET", redisKey)

	if data, e := redis.Bytes(vals, err); e != nil {
		exist = false
		return
	} else {
		if e := json.Unmarshal(data, &opts); e != nil {
			exist = false
			return
		}
		exist = true
	}

	return
}

func (p *RedisCounterStorage) SetOptions(counterName, key string, opts CounterOptions) (err error) {
	if opts == nil || len(opts) == 0 {
		return
	}

	conn := p.pool.Get()
	defer conn.Close()

	redisKey := p.redisConfig.Prefix + _CONFIG_ + counterName + ":" + key

	var js []byte
	if js, err = json.Marshal(opts); err != nil {
		return
	}

	args := []interface{}{redisKey, js}

	_, err = conn.Do("SET", args...)

	return
}
