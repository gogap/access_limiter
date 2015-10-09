package access_limiter

const (
	LimitQuotaOption = "limit_quota"
	LimitQPSOption   = "limit_qps"
)

type CounterOptions map[string]interface{}

type Counter interface {
	Name() (name string)
	Consume(count int64, dimensions ...string) (err error)
	IsCanConsume(count int64, dimensions ...string) (isCan bool)
	Reset(quota int64, dimensions ...string) (err error)
	ConsumeSpeed(dimensions ...string) (speed int64)
	UpdateOptions(opts CounterOptions, dimensions ...string) (err error)
	GetOptions(dimensions ...string) (opts CounterOptions, err error)
}
