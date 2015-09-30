package main

import (
	"fmt"
	"time"

	"github.com/gogap/access_limiter"
)

func main() {
	//storage := access_limiter.NewMemoryCounterStorage()

	redisConfig := access_limiter.RedisConfig{
		Address:   "127.0.0.1:6379",
		Password:  "",
		MaxIdle:   10000,
		MaxActive: 10000,
		Db:        1,
	}

	storage := access_limiter.NewRedisCounterStorage(redisConfig)

	counter := access_limiter.NewClassicCounter("test", storage)

	counter.UpdateOptions([]access_limiter.CounterOption{
		{access_limiter.LimitQuotaOption, "1500"},
		{access_limiter.LimitQPSOption, "500"},
	}, "1", "2", "3")

	go func(counter access_limiter.Counter) {
		i := 0

		for {

			now := time.Now().Format("2006-01-02 15:04:05")

			if err := counter.Consume(1, "1", "2", "3"); err != nil {
				fmt.Printf("\r× %s, QPS: %10d consumed: %10d.", now, counter.ConsumeSpeed("1", "2", "3"), i)
			} else {
				i += 1
				fmt.Printf("\r√ %s, QPS: %10d consumed: %10d.", now, counter.ConsumeSpeed("1", "2", "3"), i)
			}
		}
	}(counter)

	go func(counter access_limiter.Counter) {
		for {
			time.Sleep(time.Second * 10)
			fmt.Println("\nReset Quota....")
			counter.Reset("1", "2", "3")
		}
	}(counter)

	time.Sleep(time.Minute * 60)
}
