// Package safelog records request metadata without request/response dumps.
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

type routeKey struct{}
type Router struct{ httpx.Router }

// Handle injects the registered template, never a user-supplied URL or path ID.
func (router *Router) Handle(method, path string, next http.Handler) error {
	return router.Router.Handle(method, path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), routeKey{}, path)))
	}))
}

// Access runs after go-zero tracing and before application authentication. It
// never reads/mutates headers, query, body or response bytes for logging.
func Access(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		writer := &responseWriter{ResponseWriter: w}
		defer func() {
			panicked := false
			if recover() != nil {
				panicked = true
				// Panic values can contain input/credentials; don't pass them to logx.
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
func safeMethod(method string) string {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "CONNECT", "TRACE":
		return method
	default:
		return "OTHER"
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *responseWriter) WriteHeader(code int) {
	if w.status != 0 {
		return
	}
	if code >= 200 || code == http.StatusSwitchingProtocols {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}
func (w *responseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}
func (w *responseWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}
func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}
func (w *responseWriter) Push(target string, options *http.PushOptions) error {
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, options)
	}
	return http.ErrNotSupported
}
