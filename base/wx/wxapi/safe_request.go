package wxapi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/anheicang/xinlin/base/baseerror"
	"github.com/go-resty/resty/v2"
)

// Keep client construction in one place so requests have a bounded lifetime
// and tests can supply an isolated transport without contacting WeChat.
var newWXClient = func() *resty.Client {
	return resty.New().SetTimeout(30 * time.Second)
}

// Resty/net/http errors can contain the complete URL and its secret/token query.
// Do not wrap the original error: even downstream formatting must stay safe.
func wxRequest(request *resty.Request, method, requestURL string) (*resty.Response, error) {
	response, err := request.Execute(method, requestURL)
	if err == nil {
		return response, nil
	}
	kind := "transport"
	var networkError net.Error
	if errors.Is(err, context.Canceled) {
		kind = "cancelled"
	} else if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &networkError) && networkError.Timeout()) {
		kind = "timeout"
	}
	path := "<unknown>"
	if parsed, e := url.Parse(requestURL); e == nil {
		path = parsed.Path
	}
	return nil, fmt.Errorf("微信接口调用失败 method=%s path=%s kind=%s", method, path, kind)
}

func wxResponseError(code int64) error {
	// Remote errmsg may echo credentials or submitted values. Preserve the code
	// used by callers while keeping the human-readable error independent of it.
	return baseerror.NewCodeError(code, fmt.Sprintf("微信接口返回错误（code=%d），请稍后重试", code))
}

func wxDecodeError() error { return errors.New("微信接口响应格式错误") }
