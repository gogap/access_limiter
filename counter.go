package access_limiter

type OptionName string

const (
	LimitQuotaOption OptionName = "limit_quota"
	LimitQPSOption              = "limit_qps"
)

type CounterOption struct {
	Name  OptionName
	Value string
}

type Counter interface {
	Name() (name string)
	Consume(count int64, dimensions ...string) (err error)
	IsCanConsume(count int64, dimensions ...string) (isCan bool)
	Reset(quota int64, dimensions ...string) (err error)
	ConsumeSpeed(dimensions ...string) (speed int64)
	UpdateOptions(opts []CounterOption, dimensions ...string) (err error)
	GetOptions(dimensions ...string) (opts []CounterOption, err error)
}
