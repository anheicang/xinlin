// 文件用途：在保留 HTTP Writer 能力的同时记录安全访问元数据。

package safelog

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// routeKey 用私有 context key 保存路由模板，避免与业务数据冲突。
type routeKey struct{}

// Router 包装 go-zero Router，在进入 handler 前写入安全的注册路径模板。
type Router struct{ httpx.Router }

// Handle 在路由处理前注入注册时的模板路径，供日志记录使用。
// 只记录类似 /items/:id 的模板，避免记录用户实际请求路径中的标识。
func (router *Router) Handle(method, path string, next http.Handler) error {
	return router.Router.Handle(method, path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), routeKey{}, path)))
	}))
}

// Access 包装 handler，记录方法、路由模板、HTTP 状态和耗时。
// 不读取请求/响应内容；捕获 panic 时仅记录发生标记，尚未提交响应则写入 500。
func Access(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		writer := &responseWriter{ResponseWriter: w}
		defer func() {
			panicked := false
			if recover() != nil {
				panicked = true
				// panic 内容可能含请求或凭证，只记录发生标记，不把原值交给日志系统。
				if writer.status == 0 {
					writer.WriteHeader(http.StatusInternalServerError)
				}
			}
			code := writer.status
			if code == 0 {
				code = http.StatusOK
			}
			path, _ := r.Context().Value(routeKey{}).(string)
			if path == "" {
				path = "<unmatched>"
			}
			fields := []logx.LogField{logx.Field("method", safeMethod(r.Method)), logx.Field("route", path), logx.Field("status", code), logx.Field("duration_ms", time.Since(started).Milliseconds()), logx.Field("panic", panicked)}
			logger := logx.WithContext(r.Context())
			if code >= 500 || panicked {
				logger.Errorw("http_request", fields...)
			} else {
				logger.Infow("http_request", fields...)
			}
		}()
		next(writer, r)
	}
}

// safeMethod 将已知 HTTP 方法保留，其他方法统一记为 OTHER，限制日志内容来源。
func safeMethod(method string) string {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "CONNECT", "TRACE":
		return method
	default:
		return "OTHER"
	}
}

// responseWriter 记录首个最终状态，并转发写入、刷新和可选连接能力。
type responseWriter struct {
	http.ResponseWriter
	status int // 首个已提交的最终 HTTP 状态；0 表示尚未提交。
}

// Unwrap 暴露底层 ResponseWriter，供 http.ResponseController 查找可选能力。
func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// WriteHeader 记录首个最终响应状态并向底层写入；后续重复最终状态不再覆盖。
func (w *responseWriter) WriteHeader(code int) {
	if w.status != 0 {
		return
	}
	if code >= 200 || code == http.StatusSwitchingProtocols {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

// Write 写入响应正文；尚无状态时先提交默认 HTTP 200。
func (w *responseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

// Flush 提交默认状态并将缓冲内容刷新到底层，保留流式响应能力。
func (w *responseWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

// Hijack 将连接接管请求交给底层 Writer；不支持时返回 http.ErrNotSupported。
func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}

// Push 把 HTTP/2 服务端推送交给底层 Writer；不支持时返回 http.ErrNotSupported。
func (w *responseWriter) Push(target string, options *http.PushOptions) error {
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, options)
	}
	return http.ErrNotSupported
}
