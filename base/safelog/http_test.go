// 文件用途：验证访问日志不泄露请求、响应、panic 或认证信息，也不改变处理结果；验证访问日志包装保留流式输出和首个最终响应状态。

package safelog

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/router"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// capture 临时捕获 logx 输出，测试结束后恢复原日志 Writer。
func capture(t *testing.T) *bytes.Buffer {
	t.Helper()
	previous := logx.Reset()
	buffer := new(bytes.Buffer)
	logx.SetWriter(logx.NewWriter(buffer))
	t.Cleanup(func() { logx.SetWriter(previous) })
	return buffer
}

// TestAccessKeepsAuthenticationAndLogsOnlySafeMetadata 验证访问日志不泄露请求、响应、panic 或认证信息，也不改变处理结果。
func TestAccessKeepsAuthenticationAndLogsOnlySafeMetadata(t *testing.T) {
	for _, scenario := range []string{"normal", "unauthorized", "server-error", "slow", "panic"} {
		t.Run(scenario, func(t *testing.T) {
			output := capture(t)
			rt := &Router{Router: router.NewRouter()}
			code := 200
			if scenario == "unauthorized" {
				code = 401
			}
			if scenario == "server-error" || scenario == "panic" {
				code = 500
			}
			if err := rt.Handle("POST", "/items/:id", http.HandlerFunc(Access(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if r.Header.Get("Authorization") != "Bearer PRIVATE_AUTH" || r.Header.Get("Cookie") != "session=PRIVATE_COOKIE" || string(body) != "PRIVATE_BODY" || r.URL.Query().Get("token") != "PRIVATE_QUERY" {
					t.Error("logger changed request")
				}
				if scenario == "slow" {
					time.Sleep(510 * time.Millisecond)
				}
				if scenario == "panic" {
					panic("PRIVATE_PANIC")
				}
				w.Header().Set("Set-Cookie", "session=PRIVATE_RESPONSE_COOKIE")
				w.WriteHeader(code)
				_, _ = w.Write([]byte("PRIVATE_RESPONSE"))
			}))); err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest("POST", "/items/PRIVATE_PATH?token=PRIVATE_QUERY", strings.NewReader("PRIVATE_BODY"))
			request.Header.Set("Authorization", "Bearer PRIVATE_AUTH")
			request.Header.Set("Cookie", "session=PRIVATE_COOKIE")
			request.Header.Set("User-Agent", "PRIVATE_AGENT")
			traceID, _ := oteltrace.TraceIDFromHex("11223344556677889900112233445566")
			spanID, _ := oteltrace.SpanIDFromHex("1122334455667788")
			request = request.WithContext(oteltrace.ContextWithSpanContext(context.Background(), oteltrace.NewSpanContext(oteltrace.SpanContextConfig{TraceID: traceID, SpanID: spanID})))
			response := httptest.NewRecorder()
			rt.ServeHTTP(response, request)
			if response.Code != code {
				t.Fatalf("status changed: %d", response.Code)
			}
			if scenario != "panic" && response.Body.String() != "PRIVATE_RESPONSE" {
				t.Fatal("logger changed response")
			}
			logs := output.String()
			if strings.Contains(logs, "PRIVATE_") {
				t.Fatal("request, response or panic value reached log")
			}
			for _, want := range []string{"http_request", "/items/:id", "duration_ms", traceID.String()} {
				if !strings.Contains(logs, want) {
					t.Fatalf("missing metadata %s: %s", want, logs)
				}
			}
		})
	}
}

// TestAccessSupportsStreamingAndFirstFinalStatus 验证访问日志包装保留流式输出和首个最终响应状态。
func TestAccessSupportsStreamingAndFirstFinalStatus(t *testing.T) {
	output := capture(t)
	response := httptest.NewRecorder()
	Access(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("chunk"))
		w.(http.Flusher).Flush()
		w.WriteHeader(http.StatusInternalServerError)
	})(response, httptest.NewRequest("GET", "/PRIVATE_PATH?token=PRIVATE_QUERY", nil))
	if response.Code != 201 || !response.Flushed || response.Body.String() != "chunk" {
		t.Fatal("streaming/status semantics changed")
	}
	if strings.Contains(output.String(), "PRIVATE_") || !strings.Contains(output.String(), "201") {
		t.Fatal("unmatched path or status logged incorrectly")
	}
}

// TestSafeRouterPreservesCORSPreflight 验证路由包装不会破坏跨域 OPTIONS 预检。
func TestSafeRouterPreservesCORSPreflight(t *testing.T) {
	var c rest.RestConf
	if err := conf.LoadFromJsonBytes([]byte(`{"Name":"safe-log-cors-test","Host":"127.0.0.1","Port":0,"Log":{"Mode":"console"}}`), &c); err != nil {
		t.Fatal(err)
	}
	c.Middlewares.Log = false
	rt := &Router{Router: router.NewRouter()}
	server, err := rest.NewServer(c, rest.WithRouter(rt), rest.WithCors(), rest.WithNotFoundHandler(http.HandlerFunc(Access(http.NotFound))))
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()
	// Routes are normally bound during Start; register this isolated route
	// directly so the test needs no listening port or application services.
	if err := rt.Handle("POST", "/items/:id", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("OPTIONS", "/items/42", nil)
	request.Header.Set("Origin", "https://fixture.invalid")
	request.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	rt.ServeHTTP(w, request)
	if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Fatalf("CORS preflight changed: %d", w.Code)
	}
}
