// 文件用途：离线验证生图专用协议、结果边界与不重试策略，不调用真实图像模型。
package magpie

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGenerateImageProtocol(t *testing.T) {
	c := newGateway(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" || r.Method != "POST" {
			t.Errorf("unexpected route")
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["n"] != float64(1) || body["response_format"] != "b64_json" || body["prompt"] != "画一座城堡" {
			t.Errorf("unexpected image request: %+v", body)
		}
		if body["messages"] != nil || body["model"] != nil {
			t.Error("chat parameters leaked into image generation")
		}
		w.Write([]byte(`{"id":"image-1","created":123,"model":"image-model","data":[{"b64_json":"aW1hZ2U=","revised_prompt":"castle"}],"usage":{"images":1}}`))
	}, nil)
	r, err := c.GenerateImage(context.Background(), ImageRequest{Prompt: " 画一座城堡 "})
	if err != nil || r.ID != "image-1" || r.Data[0].RevisedPrompt != "castle" {
		t.Fatalf("result: %v", err)
	}
}
func TestGenerateImageBoundaries(t *testing.T) {
	for _, body := range []string{`{"data":[]}`, `{"data":[{}]}`, `{"data":[{"b64_json":"!"}]}`, `{"data":[{"url":"file:///secret"}]}`, `{"data":[{"url":"https://x.invalid/a","b64_json":"YQ=="}]}`, `{"error":{"message":"hidden vendor details"}}`} {
		t.Run(body, func(t *testing.T) {
			c := newGateway(t, func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(body)) }, nil)
			r, err := c.GenerateImage(context.Background(), ImageRequest{Prompt: "test"})
			if err == nil || r != nil {
				t.Fatal("invalid result accepted")
			}
			if strings.Contains(err.Error(), "hidden vendor details") {
				t.Fatal("vendor message leaked")
			}
		})
	}
	c := newGateway(t, func(w http.ResponseWriter, _ *http.Request) { t.Error("invalid request reached server") }, nil)
	for _, request := range []ImageRequest{{}, {Prompt: "x", N: 5}, {Prompt: "x", Quality: "invalid"}, {Prompt: "x", Background: "transparent", OutputFormat: "jpeg"}} {
		if _, err := c.GenerateImage(context.Background(), request); err == nil {
			t.Fatal("invalid options accepted")
		}
	}
}
func TestGenerateImageNoRetryAndLimit(t *testing.T) {
	var calls atomic.Int32
	c := newGateway(t, func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); w.WriteHeader(503) }, func(cfg *Config) { cfg.MaxRetries = 3 })
	if _, err := c.GenerateImage(context.Background(), ImageRequest{Prompt: "test"}); err == nil || calls.Load() != 1 {
		t.Fatal("generation was retried")
	}
	c = newGateway(t, func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(strings.Repeat("x", 100))) }, func(cfg *Config) { cfg.MaxResponseBytes = 10 })
	if _, err := c.GenerateImage(context.Background(), ImageRequest{Prompt: "test"}); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatal("response limit not enforced")
	}
}
func TestGenerateImageCancelAndURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(100 * time.Millisecond):
		}
	}))
	defer server.Close()
	c, _ := New(Config{BaseURL: server.URL, TimeoutMs: 20})
	if _, err := c.GenerateImage(context.Background(), ImageRequest{Prompt: "test"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout: %v", err)
	}
	c = newGateway(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"data":[{"url":"https://example.invalid/result.png"}]}`))
	}, nil)
	if r, err := c.GenerateImage(context.Background(), ImageRequest{Prompt: "test", ResponseFormat: "url"}); err != nil || r.Data[0].URL == "" {
		t.Fatalf("URL response: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	if _, err := c.GenerateImage(ctx, ImageRequest{Prompt: "test"}); !errors.Is(err, context.Canceled) || time.Since(started) > time.Second {
		t.Fatal("cancel not propagated")
	}
}
