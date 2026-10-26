package baseresponse_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anheicang/xinlin/base/baseerror"
	"github.com/anheicang/xinlin/base/baseresponse"
)

func TestResponseContract(t *testing.T) {
	for _, tt := range []struct {
		name   string
		err    error
		status int
		code   int64
	}{
		{"success", nil, http.StatusOK, 0},
		{"business", baseerror.NewCodeError(40001, "无效参数"), http.StatusNotAcceptable, 40001},
		{"forbidden", baseerror.NewHttpCodeError(http.StatusForbidden, 40003, "无权限"), http.StatusForbidden, 40003},
		{"wrapped forbidden", fmt.Errorf("operation: %w", baseerror.NewHttpCodeError(http.StatusForbidden, 40003, "无权限")), http.StatusForbidden, 40003},
		{"wrapped business", fmt.Errorf("operation: %w", baseerror.NewCodeError(40001, "无效参数")), http.StatusNotAcceptable, 40001},
		{"internal", errors.New("private database detail"), http.StatusInternalServerError, 10001},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/example", nil)
			baseresponse.FormatResponseWithRequest(map[string]int{"amount": 1234}, tt.err, w, r)
			var body struct {
				Code int64          `json:"code"`
				Data map[string]int `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if w.Code != tt.status || body.Code != tt.code {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if tt.err == nil && body.Data["amount"] != 1234 {
				t.Fatal("success data lost")
			}
			if strings.Contains(w.Body.String(), "private database detail") {
				t.Fatal("internal error exposed to caller")
			}
		})
	}
}

func TestBusinessErrorDataAndLegacyResponse(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		w := httptest.NewRecorder()
		err := fmt.Errorf("operation: %w", baseerror.NewError(40001, "invalid", map[string]string{"field": "name"}))
		if legacy {
			baseresponse.FormatResponse(nil, err, w)
		} else {
			baseresponse.FormatResponseWithRequest(nil, err, w, httptest.NewRequest("GET", "/", nil))
		}
		var body struct {
			Code    int64             `json:"code"`
			Data    map[string]string `json:"data"`
			Message string            `json:"message"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if w.Code != 406 || body.Code != 40001 || body.Data["field"] != "name" || body.Message != "invalid" {
			t.Fatalf("unexpected response %s", w.Body.String())
		}
	}
}
