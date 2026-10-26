package safelog

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

type pushWriter struct {
	*httptest.ResponseRecorder
	target string
}

func (w *pushWriter) Push(target string, _ *http.PushOptions) error { w.target = target; return nil }

type hijackWriter struct {
	*httptest.ResponseRecorder
	conn net.Conn
}

func (w *hijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, bufio.NewReadWriter(bufio.NewReader(w.conn), bufio.NewWriter(w.conn)), nil
}

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
