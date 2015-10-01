package access_limiter

import (
	"github.com/gogap/errors"
)

var ACCESS_LIMITER_NS = "ACCESS_LIMITER"

var (
	ERR_QPS_REACHED_UPPER_LIMIT   = errors.TN(ACCESS_LIMITER_NS, 1, "qps reached upper limit, counter: {{.counter}}, {{.dimensions}}")
	ERR_QUOTA_REACHED_UPPER_LIMIT = errors.TN(ACCESS_LIMITER_NS, 2, "quota reached upper limit, counter: {{.counter}}, {{.dimensions}}")
	ERR_INCREASE_COUNT_FAILED     = errors.TN(ACCESS_LIMITER_NS, 3, "increase counter failed, counter: {{.counter}}, err: {{.err}}")
	ERR_INCREASE_QPS_COUNT_FAILED = errors.TN(ACCESS_LIMITER_NS, 4, "increase qps count failed, counter: {{.counter}}, err: {{.err}}")
	ERR_RESET_QPS_COUNT_FAILED    = errors.TN(ACCESS_LIMITER_NS, 5, "reset next qps count failed, counter: {{.counter}}, err: {{.err}}")
	ERR_RESET_COUNT_FAILED        = errors.TN(ACCESS_LIMITER_NS, 6, "reset counter failed, counter: {{.counter}}, err: {{.err}}")
	ERR_GET_OPTIONS_FAILED        = errors.TN(ACCESS_LIMITER_NS, 7, "get options failed, counter: {{.counter}}")
	ERR_UPDATE_OPTIONS_FAILED     = errors.TN(ACCESS_LIMITER_NS, 8, "update options failed, counter: {{.counter}}, err: {{.err}}")
)
