// 文件用途：使用本地网关验证请求协议、配置、JSON 输出、错误隔离、重试和取消，不访问真实模型。
package magpie

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/core/conf"
)

const success = `{"id":"chat-1","model":"group/model","choices":[{"index":0,"message":{"role":"assistant","content":"{\"summary\":\"测试结果\"}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13}}`

// request 构造每个测试独立使用的最小请求，避免共享可变消息。
func request() ChatRequest { return ChatRequest{Messages: []Message{{Role: "user", Content: "hello"}}} }

// newGateway 创建离线 HTTP 网关和客户端，测试结束关闭连接。
func newGateway(t *testing.T, handler http.HandlerFunc, mutate func(*Config)) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	cfg := Config{BaseURL: server.URL + "/v1/", Model: "group/model", RetryDelayMs: 1}
	if mutate != nil {
		mutate(&cfg)
	}
	client, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// TestConfig 检查默认配置、go-zero 嵌套配置和构造阶段的无效值。
func TestConfig(t *testing.T) {
	c, err := New(Config{})
	if err != nil || c.cfg.BaseURL != "http://127.0.0.1:3425/v1" || c.cfg.TimeoutMs != 120000 || c.cfg.MaxResponseBytes != 2<<20 || c.cfg.RetryDelayMs != 250 {
		t.Fatalf("defaults: %+v %v", c, err)
	}
	var cfg struct{ Magpie Config }
	if err := conf.LoadFromYamlBytes([]byte("Magpie:\n  Model: group/qiqi\n  TimeoutMs: 2000\n  MaxRetries: 1\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Magpie.Model != "group/qiqi" || cfg.Magpie.TimeoutMs != 2000 || cfg.Magpie.MaxRetries != 1 {
		t.Fatalf("yaml: %+v", cfg)
	}
	for _, bad := range []Config{
		{BaseURL: "://"}, {BaseURL: "ftp://localhost"}, {BaseURL: "http://a?secret=x"}, {BaseURL: "http://user@a"}, {BaseURL: "http://a/#fragment"}, {APIKey: "key\r\nextra"},
		{TimeoutMs: -1}, {TimeoutMs: 1<<63 - 1}, {MaxRetries: -1}, {MaxRetries: 11}, {RetryDelayMs: -1}, {RetryDelayMs: 30001}, {MaxResponseBytes: -1}, {MaxResponseBytes: 1<<30 + 1},
	} {
		if _, err := New(bad); err == nil {
			t.Errorf("accepted invalid config: %+v", bad)
		}
	}
	if _, err := NewWithHTTPClient(Config{}, nil); err == nil {
		t.Fatal("nil HTTP client")
	}
}

// TestChatProtocol 验证鉴权、路径、显式零值、模型覆盖、多模态/工具及扩展参数透传。
func TestChatProtocol(t *testing.T) {
	client := newGateway(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("request: %s %s headers=%v", r.Method, r.URL.Path, r.Header)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["model"] != "another/model" || body["stream"] != false || body["temperature"] != float64(0) || body["reasoning_effort"] != "high" || body["tool_choice"] != "auto" {
			t.Errorf("body: %+v", body)
		}
		if len(body["tools"].([]any)) != 1 || len(body["messages"].([]any)) != 2 {
			t.Errorf("tools/messages: %+v", body)
		}
		fmt.Fprint(w, success)
	}, func(c *Config) { c.APIKey = " test-key " })
	zero := 0.0
	req := request()
	req.Model, req.Temperature = " another/model ", &zero
	req.Messages = append(req.Messages, Message{Role: "user", Content: []any{map[string]any{"type": "image_url", "image_url": map[string]string{"url": "https://example.invalid/image.png"}}}})
	req.Tools = []Tool{{Type: "function", Function: FunctionDefinition{Name: "lookup", Parameters: map[string]any{"type": "object"}}}}
	req.ToolChoice = "auto"
	req.Extra = map[string]any{"reasoning_effort": "high"}
	response, err := client.Chat(nil, req)
	if err != nil || response.ID != "chat-1" || response.Usage.TotalTokens != 13 {
		t.Fatalf("response: %+v %v", response, err)
	}
	if req.Model != " another/model " {
		t.Fatal("request mutated")
	}
}

// TestJSONAndModels 检查结构化输出、多种无效结果以及无需默认模型的模型发现。
func TestJSONAndModels(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		want          error
	}{
		{"ok", success, nil},
		{"bad-json", `{"choices":[{"message":{"content":"not JSON"}}]}`, ErrInvalidResponse},
		{"empty", `{"choices":[{"message":{"content":" "}}]}`, ErrNoContent},
		{"tools", `{"choices":[{"message":{"tool_calls":[{"id":"1","function":{"name":"f","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`, ErrNoContent},
		{"truncated", `{"choices":[{"message":{"content":"{}"},"finish_reason":"length"}]}`, ErrTruncated},
		{"no-choices", `{"choices":[]}`, ErrInvalidResponse},
		{"malformed", `not json`, ErrInvalidResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := newGateway(t, func(w http.ResponseWriter, r *http.Request) {
				var body ChatRequest
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body.ResponseFormat == nil || body.ResponseFormat.Type != "json_object" {
					t.Error("missing JSON format")
				}
				fmt.Fprint(w, tc.payload)
			}, nil)
			var target struct {
				Summary string `json:"summary"`
			}
			_, err := client.ChatJSON(context.Background(), request(), &target)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err=%v want=%v", err, tc.want)
			}
			if err == nil && target.Summary != "测试结果" {
				t.Fatal(target)
			}
		})
	}
	for _, payload := range []string{`{"object":"list","data":[{"id":"group/one","owned_by":"local"}]}`, `{"data":[]}`, `{}`, `broken`} {
		client := newGateway(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" || r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "" {
				t.Error("model request")
			}
			fmt.Fprint(w, payload)
		}, func(c *Config) { c.Model = "" })
		result, err := client.Models(context.Background())
		valid := strings.Contains(payload, `"data"`)
		if valid && (err != nil || result.Data == nil) || !valid && !errors.Is(err, ErrInvalidResponse) {
			t.Fatalf("models: %+v %v", result, err)
		}
	}
	client := newGateway(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, success) }, nil)
	if _, err := client.ChatJSON(nil, request(), nil); !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("nil target: %v", err)
	}
}

// TestValidationWithoutNetwork 确认参数错误不会发出请求或暴露参数内容。
func TestValidationWithoutNetwork(t *testing.T) {
	client, _ := New(Config{})
	for _, req := range []ChatRequest{
		{}, {Model: "m"}, {Model: "m", Messages: []Message{{Content: "x"}}},
		{Model: "m", Messages: []Message{{Role: "user", Content: make(chan int)}}},
		{Model: "m", Messages: request().Messages, Extra: map[string]any{"stream": true}},
		{Model: "m", Messages: request().Messages, Extra: map[string]any{"temperature": 1}},
		{Model: "m", Messages: request().Messages, Extra: map[string]any{"extension": make(chan int)}},
	} {
		if _, err := client.Chat(context.Background(), req); err == nil {
			t.Fatalf("accepted invalid request: %+v", req)
		}
	}
}

// TestHTTPErrors 检查状态分类、200 错误信封、错误脱敏、重定向和响应超限。
func TestHTTPErrors(t *testing.T) {
	for _, status := range []int{200, 400, 401, 429, 500, 502, 503, 504} {
		client := newGateway(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Request-ID", "req-1")
			w.WriteHeader(status)
			fmt.Fprint(w, `{"error":{"message":"sensitive prompt","type":"gateway","code":123}}`)
		}, nil)
		_, err := client.Chat(nil, request())
		var api *APIError
		if !errors.As(err, &api) || api.StatusCode != status || api.RequestID != "req-1" || api.Message != "sensitive prompt" || string(api.Code) != "123" || strings.Contains(err.Error(), "sensitive") {
			t.Fatalf("error: %+v %v", api, err)
		}
		if api.Retryable() != (status == 429 || status >= 500) {
			t.Fatal("retry classification")
		}
		if _, err := client.Models(nil); !errors.As(err, &api) || api.StatusCode != status {
			t.Fatalf("model API error: %v", err)
		}
	}
	client := newGateway(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/other")
		w.WriteHeader(302)
		fmt.Fprint(w, "html")
	}, nil)
	if _, err := client.Models(nil); err == nil {
		t.Fatal("redirect followed")
	}
	client = newGateway(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, success) }, func(c *Config) { c.MaxResponseBytes = 10 })
	if _, err := client.Chat(nil, request()); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatal(err)
	}
	if _, err := client.Models(nil); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatal(err)
	}
	client = newGateway(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(400); fmt.Fprint(w, success) }, func(c *Config) { c.MaxResponseBytes = 10 })
	if _, err := client.Chat(nil, request()); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatal(err)
	}
}

// TestRetryAndCancel 验证有限重试、Retry-After、总超时、父级取消和非重试状态。
func TestRetryAndCancel(t *testing.T) {
	for _, status := range []int{429, 503, 401} {
		var calls atomic.Int32
		client := newGateway(t, func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) == 1 {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(status)
				return
			}
			fmt.Fprint(w, success)
		}, func(c *Config) { c.MaxRetries = 2 })
		_, err := client.Chat(nil, request())
		if status == 401 {
			if err == nil || calls.Load() != 1 {
				t.Fatal("retried authentication failure")
			}
		} else if err != nil || calls.Load() != 2 {
			t.Fatalf("retry: %d %v", calls.Load(), err)
		}
	}
	client := newGateway(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }, func(c *Config) { c.MaxRetries = 1 })
	if _, err := client.Models(nil); err == nil {
		t.Fatal("exhaustion")
	}
	client = newGateway(t, func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Retry-After", "30"); w.WriteHeader(429) }, func(c *Config) { c.MaxRetries = 2; c.TimeoutMs = 20 })
	if _, err := client.Chat(nil, request()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("backoff deadline: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Chat(ctx, request()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, header := range []string{"", "-1", "invalid", "1000000", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat), time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)} {
		if delay := client.retryDelay(10, header); delay < 0 || delay > 30*time.Second {
			t.Fatalf("delay=%v", delay)
		}
	}
}

// transportFunc 注入网络/读取故障，避免依赖外网状态。
type transportFunc func(*http.Request) (*http.Response, error)

// RoundTrip 实现 http.RoundTripper。
func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// brokenReader 模拟响应体在读取中途失败。
type brokenReader struct{}

// Read 返回可被 errors.Is 识别的读取错误。
func (brokenReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

// TestTransportFailures 确认网络及读取失败不会自动重放生成请求。
func TestTransportFailures(t *testing.T) {
	var calls int
	client, _ := NewWithHTTPClient(Config{Model: "m", MaxRetries: 3}, &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) { calls++; return nil, io.ErrUnexpectedEOF })})
	if _, err := client.Chat(nil, request()); !errors.Is(err, io.ErrUnexpectedEOF) || calls != 1 {
		t.Fatalf("transport: %d %v", calls, err)
	}
	for _, status := range []int{200, 500} {
		client.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(brokenReader{})}, nil
		})}
		if _, err := client.Chat(nil, request()); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatal(err)
		}
		if _, err := client.Models(nil); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatal(err)
		}
	}
	// 覆盖请求构造异常；公开构造器已阻止这种非法 URL。
	client.cfg.BaseURL = "://"
	if _, err := client.Models(nil); err == nil {
		t.Fatal("invalid internal URL")
	}
}

// TestConcurrentReuse 验证共享客户端不会缓存或串用各次对话。
func TestConcurrentReuse(t *testing.T) {
	client := newGateway(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, success) }, nil)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := client.Chat(nil, request()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}
