package access_limiter

import (
	"github.com/gogap/errors"
)

var ACCESS_LIMITER_NS = "ACCESS_LIMITER"

var (
	ERR_COULD_NOT_CONSUMUE        = errors.TN(ACCESS_LIMITER_NS, 1, "could not consume, counter: {{.counter}}")
	ERR_INCREASE_COUNT_FAILED     = errors.TN(ACCESS_LIMITER_NS, 2, "increase counter failed, counter: {{.counter}}, err: {{.err}}")
	ERR_INCREASE_QPS_COUNT_FAILED = errors.TN(ACCESS_LIMITER_NS, 3, "increase qps count failed, counter: {{.counter}}, err: {{.err}}")
	ERR_RESET_QPS_COUNT_FAILED    = errors.TN(ACCESS_LIMITER_NS, 4, "reset next qps count failed, counter: {{.counter}}, err: {{.err}}")
	ERR_RESET_COUNT_FAILED        = errors.TN(ACCESS_LIMITER_NS, 5, "reset counter failed, counter: {{.counter}}, err: {{.err}}")
	ERR_GET_OPTIONS_FAILED        = errors.TN(ACCESS_LIMITER_NS, 6, "get options failed, counter: {{.counter}}")
)
