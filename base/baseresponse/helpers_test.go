// 文件用途：验证各类 HTTP 辅助响应、旧入口及 ETag 计算。

package baseresponse

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anheicang/xinlin/base/baseerror"
)

// TestResponseHelpersAndLegacyContract 验证各类 HTTP 辅助响应、旧入口及 ETag 计算。
func TestResponseHelpersAndLegacyContract(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	for _, tt := range []struct {
		write  func(http.ResponseWriter)
		status int
	}{
		{func(w http.ResponseWriter) { HttpParamError(w, errors.New("invalid")) }, 400},
		{func(w http.ResponseWriter) { HttpParamErrorWithRequest(w, r, errors.New("invalid")) }, 400},
		{func(w http.ResponseWriter) { HttpUnauthorized(w, errors.New("unauthorized")) }, 401},
		{func(w http.ResponseWriter) { HttpForbidden(w, errors.New("forbidden")) }, 403},
		{func(w http.ResponseWriter) { HttpForbidden(w, baseerror.NewCodeError(40003, "forbidden")) }, 403},
		{func(w http.ResponseWriter) { FormatResponse("ok", nil, w) }, 200},
		{func(w http.ResponseWriter) { FormatResponse(nil, errors.New("internal"), w) }, 500},
		{func(w http.ResponseWriter) {
			FormatResponse(nil, baseerror.NewHttpCodeError(409, 40009, "conflict"), w)
		}, 409},
	} {
		w := httptest.NewRecorder()
		tt.write(w)
		if w.Code != tt.status {
			t.Fatalf("status=%d want=%d", w.Code, tt.status)
		}
	}
	if calculateETag("same") != calculateETag("same") || calculateETag("same") == calculateETag("different") || calculateETag(make(chan int)) != "" {
		t.Fatal("ETag contract incorrect")
	}
}
