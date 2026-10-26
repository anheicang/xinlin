// 文件用途：验证底层 JSON 字段、HTTP 状态和 Content-Type。

package proto

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHTTPResponses 验证底层 JSON 字段、HTTP 状态和 Content-Type。
func TestHTTPResponses(t *testing.T) {
	for _, tt := range []struct {
		name   string
		write  func(http.ResponseWriter)
		status int
		code   int64
		data   interface{}
	}{
		{"ok", func(w http.ResponseWriter) { HttpOk(w, "value") }, 200, 0, "value"},
		{"business", func(w http.ResponseWriter) { HttpError(w, 409, 40009, "conflict", "details") }, 409, 40009, "details"},
		{"parameter", func(w http.ResponseWriter) { HttpParamError(w, "invalid") }, 400, 400, nil},
		{"unauthorized", func(w http.ResponseWriter) { HttpUnauthorized(w, "login required") }, 401, 401, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tt.write(w)
			var got response
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if w.Code != tt.status || got.Code != tt.code || got.Data != tt.data || w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
				t.Fatalf("invalid response: status=%d body=%s headers=%v", w.Code, w.Body.String(), w.Header())
			}
			if tt.status != 200 && got.Desc == "" {
				t.Fatal("missing error description")
			}
		})
	}
}
