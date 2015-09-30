package access_limiter

import (
	"github.com/gogap/errors"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ClassicCounter struct {
	name    string
	storage CounterStorage

	cacheLocker     sync.Mutex
	cachedOptions   map[string][]CounterOption
	latestCacheTime time.Time
}

func NewClassicCounter(name string, storage CounterStorage) Counter {
	return &ClassicCounter{
		name:            name,
		storage:         storage,
		cachedOptions:   make(map[string][]CounterOption),
		latestCacheTime: time.Now(),
	}
}

func (p *ClassicCounter) Name() (name string) {
	return p.name
}

func (p *ClassicCounter) Consume(count int64, dimensions ...string) (err error) {
	if !p.IsCanConsume(count, dimensions...) {
		err = ERR_COULD_NOT_CONSUMUE.New(errors.Params{"counter": p.name})
		return
	}

	if e := p.storage.Increase(p.name, count, dimensions...); e != nil {
		err = ERR_INCREASE_COUNT_FAILED.New(errors.Params{"counter": p.name, "err": e})
		return
	}

	nowSec := time.Now().Second()
	index := nowSec % 5
	nextIndex := (nowSec + 1) % 5

	dimPrefix := ""
	if dimensions != nil {
		dimPrefix = strings.Join(dimensions, ":")
	}

	qpsDims := []string{dimPrefix, "qps", strconv.Itoa(index)}

	if e := p.storage.Increase(p.name, 1, qpsDims...); e != nil {
		err = ERR_INCREASE_QPS_COUNT_FAILED.New(errors.Params{"counter": p.name, "err": e})
		return
	}

	nextQPSDims := []string{dimPrefix, "qps", strconv.Itoa(nextIndex)}
	if e := p.storage.SetValue(p.name, 0, nextQPSDims...); e != nil {
		err = ERR_RESET_QPS_COUNT_FAILED.New(errors.Params{"counter": p.name, "err": e})
		return
	}

	return
}

func (p *ClassicCounter) IsCanConsume(count int64, dimensions ...string) (isCan bool) {
	if opts, e := p.GetOptions(dimensions...); e != nil {
		isCan = false
		return
	} else {
		isCan = true
		for _, opt := range opts {
			if opt.Name == "limit_quota" {
				optVal, _ := strconv.ParseInt(opt.Value, 10, 64)

				isCan = optVal == -1

				dimV, _ := p.storage.GetValue(p.name, dimensions...)

				isCan = dimV+count <= optVal
			} else if opt.Name == "limit_qps" {
				optVal, _ := strconv.ParseInt(opt.Value, 10, 64)

				dimPrefix := ""
				if optVal > 0 {

					if dimensions != nil {
						dimPrefix = strings.Join(dimensions, ":")
					}

					dimGroup := p.dimensionGroup(dimPrefix)

					sumV, _ := p.storage.GetSumValue(p.name, dimGroup)

					isCan = (sumV+count)/int64(len(dimGroup)-1) <= optVal
				} else {
					isCan = true
				}

			}

			if !isCan {
				dimPrefix := ""

				if dimensions != nil {
					dimPrefix = strings.Join(dimensions, ":")
				}

				nowSec := time.Now().Second()
				nextIndex := (nowSec + 1) % 5
				nextQPSDims := []string{dimPrefix, "qps", strconv.Itoa(nextIndex)}
				p.storage.SetValue(p.name, 0, nextQPSDims...)
				return
			}
		}
	}

	return
}

func (p *ClassicCounter) Reset(dimensions ...string) (err error) {
	if e := p.storage.Delete(p.name, dimensions...); e != nil {
		err = ERR_RESET_COUNT_FAILED.New(errors.Params{"counter": p.name, "err": e})
		return
	}
	return
}

func (p *ClassicCounter) dimensionGroup(prefix string) [][]string {
	return [][]string{
		{prefix, "qps:0"},
		{prefix, "qps:1"},
		{prefix, "qps:2"},
		{prefix, "qps:3"},
		{prefix, "qps:4"}}
}

func (p *ClassicCounter) ConsumeSpeed(dimensions ...string) (speed int64) {
	dimPrefix := ""
	if dimensions != nil {
		dimPrefix = strings.Join(dimensions, ":")
	}

	dimGroup := p.dimensionGroup(dimPrefix)

	sumV, _ := p.storage.GetSumValue(p.name, dimGroup)

	speed = sumV / int64(len(dimGroup)-1)

	return
}

func (p *ClassicCounter) UpdateOptions(opts []CounterOption, dimensions ...string) (err error) {
	optKey := ""
	if dimensions != nil {
		optKey = strings.Join(dimensions, ":")
	}

	p.storage.SetOptions(p.name, optKey, opts...)

	return
}

func (p *ClassicCounter) GetOptions(dimensions ...string) (opts []CounterOption, err error) {
	optKey := ""
	if dimensions != nil {
		optKey = strings.Join(dimensions, ":")
	}

	cacheTimeUp := int32(time.Now().Sub(p.latestCacheTime).Seconds()) >= 10

	if cacheTimeUp {
		p.latestCacheTime = time.Now()
	}

	if v, exist := p.cachedOptions[optKey]; exist && !cacheTimeUp {
		opts = v
		return
	}

	if v, exist := p.storage.GetOption(p.name, optKey); exist {
		p.cacheLocker.Lock()
		defer p.cacheLocker.Unlock()

		p.cachedOptions[optKey] = v
		opts = v
		return
	} else if v, exist := p.cachedOptions[optKey]; exist {
		opts = v
		return
	} else {
		err = ERR_GET_OPTIONS_FAILED.New(errors.Params{"counter": p.name})
		return
	}
	return
}
