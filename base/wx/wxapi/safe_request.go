// 文件用途：集中设置请求超时，并避免微信 URL 中的凭证被带入错误日志。

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

// newWXClient 为微信请求统一设置 30 秒超时；测试可替换工厂以隔离真实网络。
var newWXClient = func() *resty.Client {
	return resty.New().SetTimeout(30 * time.Second)
}

// wxRequest 执行微信 HTTP 请求，并将网络错误转换为仅含方法、路径和错误类别的提示。
// 不包装原始错误，避免下游日志展开 URL 中的 secret、token 或 code。
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

// wxResponseError 保留微信业务错误码，使用固定描述替换可能包含敏感输入的上游 errmsg。
func wxResponseError(code int64) error {
	// 微信 errmsg 可能回显凭证或输入值，保留可供调用方判断的错误码，并重建描述。
	return baseerror.NewCodeError(code, fmt.Sprintf("微信接口返回错误（code=%d），请稍后重试", code))
}

// wxDecodeError 返回统一的微信响应格式错误，避免对外展示原始响应内容。
func wxDecodeError() error { return errors.New("微信接口响应格式错误") }
