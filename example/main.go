package main

import (
	"fmt"
	"time"

	"github.com/gogap/access_limiter"
)

func main() {
	//storage := access_limiter.NewMemoryCounterStorage()

	redisConfig := access_limiter.RedisConfig{
		Address:         "127.0.0.1:6379",
		Password:        "",
		MaxIdle:         10000,
		MaxActive:       10000,
		Db:              1,
		Transaction:     true,
		ConsistentRetry: 3,
	}

	storage := access_limiter.NewRedisCounterStorage(redisConfig)

	counter := access_limiter.NewClassicCounter("test", storage)

	counter.UpdateOptions([]access_limiter.CounterOption{
		{access_limiter.LimitQuotaOption, "15000"},
		{access_limiter.LimitQPSOption, "1000"},
	}, "shoes", "oid-001")

	go func(counter access_limiter.Counter) {
		i := 0

		for {
			now := time.Now().Format("2006-01-02 15:04:05")

			if err := counter.Consume(1, "shoes", "oid-001"); err != nil {
				fmt.Printf("\r× %s, QPS: %10d consumed: %10d.", now, counter.ConsumeSpeed("shoes", "oid-001"), i)
			} else {
				i += 1
				fmt.Printf("\r√ %s, QPS: %10d consumed: %10d.", now, counter.ConsumeSpeed("shoes", "oid-001"), i)
			}
		}
	}(counter)

	go func(counter access_limiter.Counter) {
		for {
			time.Sleep(time.Second * 20)
			fmt.Println("\nReset Quota....")
			counter.Reset(0, "shoes", "oid-001")
		}
	}(counter)

	go func() {
		for {
			time.Sleep(time.Second * 1)
			fmt.Println("")
		}
	}()

	time.Sleep(time.Minute * 60)
}
