// 文件用途：验证日志 Writer 兼容流式刷新、连接接管、HTTP/2 推送和状态记录。

package safelog

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// pushWriter 记录推送目标的 Writer 替身，用于测试可选 Push 能力。
type pushWriter struct {
	*httptest.ResponseRecorder
	target string
}

// Push 记录服务端推送目标，验证 Writer 包装保留可选接口。
func (w *pushWriter) Push(target string, _ *http.PushOptions) error { w.target = target; return nil }

// hijackWriter 返回内存连接的 Writer 替身，用于测试 Hijack 转发。
type hijackWriter struct {
	*httptest.ResponseRecorder
	conn net.Conn
}

// Hijack 返回模拟连接，验证 Writer 包装保留连接接管能力。
func (w *hijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, bufio.NewReadWriter(bufio.NewReader(w.conn), bufio.NewWriter(w.conn)), nil
}

// TestWriterOptionalInterfacesAndStatus 验证日志 Writer 兼容流式刷新、连接接管、HTTP/2 推送和状态记录。
func TestWriterOptionalInterfacesAndStatus(t *testing.T) {
	base := httptest.NewRecorder()
	w := &responseWriter{ResponseWriter: base}
	if w.Unwrap() != base {
		t.Fatal("underlying writer lost")
	}
	if _, _, err := w.Hijack(); !errors.Is(err, http.ErrNotSupported) {
		t.Fatalf("unsupported hijack=%v", err)
	}
	if err := w.Push("/asset", nil); !errors.Is(err, http.ErrNotSupported) {
		t.Fatal("unsupported push hidden")
	}
	w.Flush()
	if !base.Flushed || w.status != 200 {
		t.Fatal("flush did not commit status")
	}
	pusher := &pushWriter{ResponseRecorder: httptest.NewRecorder()}
	if err := (&responseWriter{ResponseWriter: pusher}).Push("/asset", nil); err != nil || pusher.target != "/asset" {
		t.Fatal("push delegation lost")
	}
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	conn, _, err := (&responseWriter{ResponseWriter: &hijackWriter{ResponseRecorder: httptest.NewRecorder(), conn: left}}).Hijack()
	if err != nil || conn != left {
		t.Fatal("hijack delegation lost")
	}
	if safeMethod("CUSTOM") != "OTHER" || safeMethod("GET") != "GET" {
		t.Fatal("method allowlist incorrect")
	}
	capture(t)
	Access(func(http.ResponseWriter, *http.Request) {})(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	Access(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("body")) })(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
}
