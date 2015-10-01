package access_limiter

import (
	"github.com/gogap/errors"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	_CONSUME_ = ":_consume_"
	_QPS_     = ":_qps_"
)

type ClassicCounter struct {
	name    string
	storage CounterStorage

	cacheLocker      sync.Mutex
	cachedOptions    map[string][]CounterOption
	cachedDimOptions map[string]string
	latestCacheTime  time.Time
}

func NewClassicCounter(name string, storage CounterStorage) Counter {
	return &ClassicCounter{
		name:             name,
		storage:          storage,
		cachedOptions:    make(map[string][]CounterOption),
		cachedDimOptions: make(map[string]string),
		latestCacheTime:  time.Now(),
	}
}

func (p *ClassicCounter) Name() (name string) {
	return p.name
}

func (p *ClassicCounter) Consume(count int64, dimensions ...string) (err error) {

	defer func() {
		go p.updateQPSCounter(err != nil, dimensions...)
	}()

	if p.isReachedQPSUpperLimit(dimensions...) {
		err = ERR_QPS_REACHED_UPPER_LIMIT.New(errors.Params{"counter": p.name, "dimensions": strings.Join(dimensions, ":")})
		return
	}

	var maxQuota int64 = 0
	if v, exist := p.getDimensionOption(LimitQuotaOption, dimensions...); exist {
		maxQuota, _ = strconv.ParseInt(v, 10, 64)
	}

	if e := p.storage.Increase(p.name+_CONSUME_, count, maxQuota, dimensions...); e != nil {
		if errors.IsErrCode(e) {
			err = e
		} else {
			err = ERR_INCREASE_COUNT_FAILED.New(errors.Params{"counter": p.name, "err": e})
		}
		return
	}

	return
}

func (p *ClassicCounter) IsCanConsume(count int64, dimensions ...string) (isCan bool) {

	isCan = true

	go p.updateQPSCounter(true, dimensions...)

	if p.isReachedQuotaUpperLimit(dimensions...) {
		isCan = false
		return
	}

	if p.isReachedQPSUpperLimit(dimensions...) {
		isCan = false
		return
	}

	return
}

func (p *ClassicCounter) Reset(dimensions ...string) (err error) {
	if e := p.storage.Delete(p.name+_CONSUME_, dimensions...); e != nil {
		err = ERR_RESET_COUNT_FAILED.New(errors.Params{"counter": p.name, "err": e})
		return
	}
	return
}

func (p *ClassicCounter) dimensionGroup(prefix string) [][]string {
	return [][]string{
		{prefix, "0"},
		{prefix, "1"},
		{prefix, "2"},
		{prefix, "3"},
		{prefix, "4"}}
}

func (p *ClassicCounter) ConsumeSpeed(dimensions ...string) (speed int64) {
	dimPrefix := ""
	if dimensions != nil {
		dimPrefix = strings.Join(dimensions, ":")
	}

	dimGroup := p.dimensionGroup(dimPrefix)

	sumV, _ := p.storage.GetSumValue(p.name+_QPS_, dimGroup)

	speed = sumV / int64(len(dimGroup)-1)

	return
}

func (p *ClassicCounter) UpdateOptions(opts []CounterOption, dimensions ...string) (err error) {
	optKey := ""
	if dimensions != nil {
		optKey = strings.Join(dimensions, ":")
	}

	if e := p.storage.SetOptions(p.name, optKey, opts...); e != nil {
		err = ERR_UPDATE_OPTIONS_FAILED.New(errors.Params{"counter": p.name, "err": e})
		return
	} else {
		p.cacheLocker.Lock()
		defer p.cacheLocker.Unlock()

		p.cachedOptions[optKey] = opts

		for _, opt := range opts {
			p.cachedDimOptions[optKey+":"+string(opt.Name)] = opt.Value
		}
	}

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

	if v, exist := p.storage.GetOptions(p.name, optKey); exist {
		p.cacheLocker.Lock()
		defer p.cacheLocker.Unlock()

		p.cachedOptions[optKey] = v

		for _, opt := range v {
			p.cachedDimOptions[optKey+":"+string(opt.Name)] = opt.Value
		}

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

func (p *ClassicCounter) getDimensionOption(optName OptionName, dimensions ...string) (v string, exist bool) {
	optKey := ""
	if dimensions != nil {
		optKey = strings.Join(dimensions, ":")
	}

	v, exist = p.cachedDimOptions[optKey+":"+string(optName)]

	return
}

func (p *ClassicCounter) isReachedQPSUpperLimit(dimensions ...string) bool {
	var optVal int64 = 0

	if strOptv, exist := p.getDimensionOption(LimitQPSOption, dimensions...); !exist {
		return false
	} else {
		optVal, _ = strconv.ParseInt(strOptv, 10, 64)
	}

	if optVal > 0 {
		return p.ConsumeSpeed(dimensions...) > optVal
	}

	return false
}

func (p *ClassicCounter) isReachedQuotaUpperLimit(dimensions ...string) bool {
	var optVal int64 = 0

	if strOptv, exist := p.getDimensionOption(LimitQuotaOption, dimensions...); !exist {
		return false
	} else {
		optVal, _ = strconv.ParseInt(strOptv, 10, 64)
	}

	if optVal == -1 {
		return false
	}

	dimV, _ := p.storage.GetValue(p.name+_CONSUME_, dimensions...)

	return dimV > optVal
}

func (p *ClassicCounter) updateQPSCounter(resetOnly bool, dimensions ...string) {
	nowSec := time.Now().Second()
	index := nowSec % 5

	dimPrefix := ""
	if dimensions != nil {
		dimPrefix = strings.Join(dimensions, ":")
	}

	if !resetOnly {
		qpsDims := []string{dimPrefix, strconv.Itoa(index)}

		if e := p.storage.Increase(p.name+_QPS_, 1, 0, qpsDims...); e != nil {
			return
		}
	}

	nextIndex := (time.Now().Second() + 1) % 5
	nextQPSDims := []string{dimPrefix, strconv.Itoa(nextIndex)}
	p.storage.SetValue(p.name+_QPS_, 0, nextQPSDims...)
}
