package access_limiter

import (
	"github.com/gogap/errors"
	"strings"
	"sync"
)

type MemoryCounterStorage struct {
	counterLocker sync.Mutex
	optionLocker  sync.Mutex

	counter map[string]map[string]int64
	kv      map[string]CounterOptions
}

func NewMemoryCounterStorage() CounterStorage {
	return &MemoryCounterStorage{
		counter: make(map[string]map[string]int64),
		kv:      make(map[string]CounterOptions),
	}
}

func (p *MemoryCounterStorage) ensureCounterExist(counterName string) {
	if _, exist := p.counter[counterName]; !exist {
		p.counter[counterName] = make(map[string]int64)
	}
}

func (p *MemoryCounterStorage) Increase(counterName string, count, max int64, dimensions ...string) (err error) {
	p.counterLocker.Lock()
	defer p.counterLocker.Unlock()

	p.ensureCounterExist(counterName)

	dims, _ := p.counter[counterName]

	dimKey := ""
	if dimensions != nil {
		dimKey = strings.Join(dimensions, ":")
	}

	if max > 0 && dims[dimKey]+count > max {
		err = ERR_QUOTA_REACHED_UPPER_LIMIT.New(errors.Params{"counter": counterName, "dimensions": strings.Join(dimensions, ":")})
		return
	}

	dims[dimKey] = dims[dimKey] + count

	p.counter[counterName] = dims

	return
}

func (p *MemoryCounterStorage) Delete(counterName string, dimensions ...string) (err error) {
	p.counterLocker.Lock()
	defer p.counterLocker.Unlock()

	if _, exist := p.counter[counterName]; !exist {
		return
	}

	if dimensions == nil || len(dimensions) == 0 {
		delete(p.counter, counterName)
	}

	dims, _ := p.counter[counterName]

	dimKey := strings.Join(dimensions, ":")

	delete(dims, dimKey)

	p.counter[counterName] = dims

	return
}

func (p *MemoryCounterStorage) SetValue(counterName string, value int64, dimensions ...string) (err error) {
	p.counterLocker.Lock()
	defer p.counterLocker.Unlock()

	p.ensureCounterExist(counterName)

	dims, _ := p.counter[counterName]

	dimKey := ""
	if dimensions != nil {
		dimKey = strings.Join(dimensions, ":")
	}

	dims[dimKey] = value

	p.counter[counterName] = dims

	return
}

func (p *MemoryCounterStorage) GetValue(counterName string, dimensions ...string) (dimVal int64, exist bool) {

	if _, exist = p.counter[counterName]; !exist {
		return
	}

	dims, _ := p.counter[counterName]

	dimKey := ""
	if dimensions != nil {
		dimKey = strings.Join(dimensions, ":")
	}

	dimVal, exist = dims[dimKey]

	return
}

func (p *MemoryCounterStorage) GetSumValue(counterName string, dimensionsGroup [][]string) (sumDimVal int64, err error) {
	if _, exist := p.counter[counterName]; !exist {
		return
	}

	if dimensionsGroup == nil {
		return
	}

	dims, _ := p.counter[counterName]

	var totalVal int64 = 0

	for _, dim := range dimensionsGroup {
		dimKey := ""
		if dim != nil {
			dimKey = strings.Join(dim, ":")
		}

		dimV, _ := dims[dimKey]

		totalVal += dimV
	}

	sumDimVal = totalVal

	return
}

func (p *MemoryCounterStorage) GetOptions(counterName, key string) (opts CounterOptions, exist bool) {
	opts, exist = p.kv[counterName+":"+key]
	return
}

func (p *MemoryCounterStorage) SetOptions(counterName, key string, opts CounterOptions) (err error) {
	if opts == nil {
		return
	}

	p.optionLocker.Lock()
	defer p.optionLocker.Unlock()

	p.kv[counterName+":"+key] = opts

	return
}
