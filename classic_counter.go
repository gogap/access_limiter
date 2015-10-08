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

	cacheLocker        sync.Mutex
	cachedOptions      map[string][]CounterOption
	cachedDimOptions   map[string]string
	latestOptCacheTime time.Time

	qpsCacheLocker sync.Mutex
	cachedQPSCount map[string]int64
	cachedQPS      map[string]int64
}

func NewClassicCounter(name string, storage CounterStorage) Counter {
	counter := &ClassicCounter{
		name:               name,
		storage:            storage,
		cachedOptions:      make(map[string][]CounterOption),
		cachedDimOptions:   make(map[string]string),
		latestOptCacheTime: time.Now(),
		cachedQPSCount:     make(map[string]int64),
		cachedQPS:          make(map[string]int64),
	}

	counter.beginSyncQPSCounter()

	return counter
}

func (p *ClassicCounter) Name() (name string) {
	return p.name
}

func (p *ClassicCounter) Consume(count int64, dimensions ...string) (err error) {

	defer func() {
		go p.increaseQPSCount(1, dimensions...)
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

func (p *ClassicCounter) Reset(quota int64, dimensions ...string) (err error) {
	if quota <= 0 {
		if e := p.storage.Delete(p.name+_CONSUME_, dimensions...); e != nil {
			err = ERR_RESET_COUNT_FAILED.New(errors.Params{"counter": p.name, "err": e})
			return
		}
	} else {
		if e := p.storage.SetValue(p.name+_CONSUME_, quota, dimensions...); e != nil {
			err = ERR_RESET_COUNT_FAILED.New(errors.Params{"counter": p.name, "err": e})
			return
		}
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

	speed, _ = p.cachedQPS[dimPrefix]

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

	cacheTimeUp := int32(time.Now().Sub(p.latestOptCacheTime).Seconds()) >= 10

	if cacheTimeUp {
		p.latestOptCacheTime = time.Now()
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

func (p *ClassicCounter) beginSyncQPSCounter() {
	go func() {
		for {
			time.Sleep(time.Second)
			p.syncQPSCounter()
		}
	}()
}

func (p *ClassicCounter) syncQPSCounter() {
	nowSec := time.Now().Second()
	index := nowSec % 5

	nextIndex := (time.Now().Second() + 1) % 5

	for dimPrefix, val := range p.cachedQPSCount {
		qpsDims := []string{dimPrefix, strconv.Itoa(index)}
		if e := p.storage.Increase(p.name+_QPS_, val, 0, qpsDims...); e != nil {
			continue
		}

		nextQPSDims := []string{dimPrefix, strconv.Itoa(nextIndex)}
		p.storage.SetValue(p.name+_QPS_, 0, nextQPSDims...)

		dimGroup := p.dimensionGroup(dimPrefix)

		sumV, _ := p.storage.GetSumValue(p.name+_QPS_, dimGroup)

		p.cachedQPS[dimPrefix] = sumV / int64(len(dimGroup)-1)
	}
	p.clearQPSCount()
}

func (p *ClassicCounter) increaseQPSCount(count int64, dimensions ...string) {
	key := ""
	if dimensions != nil {
		key = strings.Join(dimensions, ":")
	}

	p.qpsCacheLocker.Lock()
	defer p.qpsCacheLocker.Unlock()

	if val, exist := p.cachedQPSCount[key]; exist {
		val += count
		p.cachedQPSCount[key] = val
	} else {
		p.cachedQPSCount[key] = count
	}
}

func (p *ClassicCounter) clearQPSCount() {
	p.qpsCacheLocker.Lock()
	defer p.qpsCacheLocker.Unlock()

	for k, _ := range p.cachedQPSCount {
		p.cachedQPSCount[k] = 0
	}
}
