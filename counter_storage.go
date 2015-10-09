package access_limiter

type CounterStorage interface {
	Increase(counterName string, count, max int64, dimensions ...string) (err error)
	Delete(counterName string, dimensions ...string) (err error)
	SetValue(counterName string, value int64, dimensions ...string) (err error)
	GetValue(counterName string, dimensions ...string) (dimVal int64, exist bool)
	GetSumValue(counterName string, dimensionsGroup [][]string) (sumDimVal int64, err error)
	GetOptions(counterName, key string) (opts CounterOptions, exist bool)
	SetOptions(counterName, key string, opts CounterOptions) (err error)
}
