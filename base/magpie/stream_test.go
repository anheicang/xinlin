// 文件用途：验证 SSE 的增量协议、工具/用量事件、结束标记、异常断流和回调取消。
package magpie

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// TestStreamProtocol 检查多行事件、CRLF、心跳、工具增量和独立用量事件。
func TestStreamProtocol(t *testing.T) {
	client := newGateway(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["stream"] != true || !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			t.Error("missing stream flag/accept")
		}
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		fmt.Fprint(w, ": heartbeat\r\n\r\nevent: message\r\nid: 1\r\ndata: {\"choices\":[\r\ndata: {\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"你\"}}]}\r\n\r\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"好\",\"reasoning_content\":\"思考\",\"tool_calls\":[{\"index\":0,\"id\":\"call1\",\"function\":{\"name\":\"lookup\",\"arguments\":\"{}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"total_tokens\":9}}\n\ndata: [DONE]\n\n")
	}, nil)
	var text string
	var chunks []ChatChunk
	err := client.Stream(nil, request(), func(chunk ChatChunk) error {
		chunks = append(chunks, chunk)
		for _, choice := range chunk.Choices {
			if content, ok := choice.Delta.Content.(string); ok {
				text += content
			}
		}
		return nil
	})
	if err != nil || text != "你好" || len(chunks) != 3 || chunks[2].Usage.TotalTokens != 9 || chunks[1].Choices[0].Delta.ToolCalls[0].Function.Name != "lookup" {
		t.Fatalf("stream: %+v %v", chunks, err)
	}
}

// TestStreamFailures 检查非 SSE 响应、错误信封、超限、丢失 DONE 和读取失败。
func TestStreamFailures(t *testing.T) {
	for _, tc := range []struct {
		name, contentType, body string
		limit                   int64
		want                    error
	}{
		{"wrong-type", "application/json", success, 0, ErrInvalidResponse},
		{"malformed", "text/event-stream", "data: broken\n\n", 0, ErrInvalidResponse},
		{"unknown", "text/event-stream", "data: {}\n\n", 0, ErrInvalidResponse},
		{"no-done", "text/event-stream", "data: {\"choices\":[]}\n\n", 0, ErrIncompleteStream},
		{"no-final-delimiter", "text/event-stream", "data: [DONE]", 0, ErrIncompleteStream},
		{"empty-data", "text/event-stream", "data\n\ndata: [DONE]\n\n", 0, ErrInvalidResponse},
		{"too-big-json", "application/json", success, 20, ErrResponseTooLarge},
		{"too-big-line", "text/event-stream", "data: " + strings.Repeat("x", 5000) + "\n\n", 20, ErrResponseTooLarge},
		{"too-big-buffered-line", "text/event-stream", "data: " + strings.Repeat("x", 50) + "\n\n", 20, ErrResponseTooLarge},
		{"too-big-event", "text/event-stream", strings.Repeat("data: 12345\n", 4) + "\n", 20, ErrResponseTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := newGateway(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				fmt.Fprint(w, tc.body)
			}, func(c *Config) { c.MaxResponseBytes = tc.limit })
			if err := client.Stream(nil, request(), func(ChatChunk) error { return nil }); !errors.Is(err, tc.want) {
				t.Fatalf("err=%v want=%v", err, tc.want)
			}
		})
	}
	for _, contentType := range []string{"text/event-stream", "application/json"} {
		body := `{"error":{"message":"private","code":"denied"}}`
		if contentType == "text/event-stream" {
			body = "data: " + body + "\n\n"
		}
		client := newGateway(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", contentType)
			fmt.Fprint(w, body)
		}, nil)
		var api *APIError
		if err := client.Stream(nil, request(), func(ChatChunk) error { return nil }); !errors.As(err, &api) || api.StatusCode != 200 {
			t.Fatalf("error event: %v", err)
		}
	}
	for _, contentType := range []string{"text/event-stream", "application/json"} {
		client, _ := NewWithHTTPClient(Config{Model: "m"}, &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{contentType}}, Body: io.NopCloser(brokenReader{})}, nil
		})})
		if err := client.Stream(nil, request(), func(ChatChunk) error { return nil }); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatal(err)
		}
	}
}

// TestStreamStop 验证业务回调报错后立即停止、HTTP 错误不重试及父 context 的取消。
func TestStreamStop(t *testing.T) {
	stop := errors.New("stop receiver")
	client := newGateway(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[]}\n\ndata: [DONE]\n\n")
	}, nil)
	if err := client.Stream(nil, request(), func(ChatChunk) error { return stop }); !errors.Is(err, stop) {
		t.Fatal(err)
	}
	if err := client.Stream(nil, request(), nil); err == nil {
		t.Fatal("nil receiver")
	}
	if err := client.Stream(nil, ChatRequest{}, func(ChatChunk) error { return nil }); err == nil {
		t.Fatal("invalid request")
	}
	var calls atomic.Int32
	client = newGateway(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(503) }, func(c *Config) { c.MaxRetries = 3 })
	if err := client.Stream(nil, request(), func(ChatChunk) error { return nil }); err == nil || calls.Load() != 1 {
		t.Fatalf("stream retried: %v %d", err, calls.Load())
	}
	client = newGateway(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}, func(c *Config) { c.TimeoutMs = 20 })
	if err := client.Stream(nil, request(), func(ChatChunk) error { return nil }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stream deadline: %v", err)
	}
}
