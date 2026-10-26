// 文件用途：验证快递查询参数、URL、网络、HTTP、响应读取和业务失败；验证物流状态、签收标记、无效时间及最早/最晚轨迹提取。

package kd100

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// transportFunc 用于离线快递查询的 HTTP 传输函数适配器。
type transportFunc func(*http.Request) (*http.Response, error)

// RoundTrip 将 HTTP 请求交给测试处理函数，不访问真实供应商接口。
func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// badBody 模拟响应正文读取失败，供错误传播测试使用。
type badBody struct{}

// Read 主动模拟读取失败，用于验证上层错误传播。
func (badBody) Read([]byte) (int, error) { return 0, errors.New("body unreadable") }

// Close 完成测试替身的无副作用资源关闭。
func (badBody) Close() error { return nil }

// TestQueryValidationAndUpstreamFailures 验证快递查询参数、URL、网络、HTTP、响应读取和业务失败。
func TestQueryValidationAndUpstreamFailures(t *testing.T) {
	request := NewQueryRequest("ems", "123456", "")
	var disabled *Client
	if _, err := disabled.QueryExpress(nil, request); err == nil {
		t.Fatal("nil client accepted")
	}
	client := NewClientWithHTTPClient(" customer ", " key ", "https://fixture.invalid", nil)
	if !client.Enabled() || client.httpClient.Timeout <= 0 {
		t.Fatal("client defaults incorrect")
	}
	for _, req := range []*QueryRequest{nil, {}, NewQueryRequest("ems", strings.Repeat("1", 33), "")} {
		if _, err := client.QueryExpress(nil, req); err == nil {
			t.Fatal("invalid query accepted")
		}
	}
	for _, tt := range []struct {
		name   string
		status int
		body   string
		err    error
		reader io.ReadCloser
	}{
		{"http", 503, strings.Repeat("x", 3000), nil, nil},
		{"decode", 200, "not-json", nil, nil},
		{"business", 200, `{"status":"500","message":"not found"}`, nil, nil},
		{"transport", 0, "", errors.New("network unavailable"), nil},
		{"read", 200, "", nil, badBody{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client.httpClient = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
				if tt.err != nil {
					return nil, tt.err
				}
				body := tt.reader
				if body == nil {
					body = io.NopCloser(strings.NewReader(tt.body))
				}
				return &http.Response{StatusCode: tt.status, Header: make(http.Header), Body: body}, nil
			})}
			result, err := client.QueryExpress(nil, &QueryRequest{Company: "ems", Number: "123456"})
			if err == nil {
				t.Fatal("query failure hidden")
			}
			if tt.name == "business" && (result == nil || result.APIStatus() != "500") {
				t.Fatal("business response lost")
			}
			if tt.name == "http" && len(err.Error()) > 2300 {
				t.Fatal("error body not bounded")
			}
		})
	}
	client.apiURL = "://bad-url"
	if _, err := client.QueryExpress(context.Background(), request); err == nil {
		t.Fatal("invalid URL accepted")
	}
}

// TestTrackingStatesAndTimes 验证物流状态、签收标记、无效时间及最早/最晚轨迹提取。
func TestTrackingStatesAndTimes(t *testing.T) {
	var empty *QueryResponse
	if empty.APIStatus() != "" || empty.IsSuccess() || empty.IsSigned() {
		t.Fatal("nil response semantics incorrect")
	}
	response := &QueryResponse{Status: "500", ReturnCode: "200", Message: "ok"}
	if response.APIStatus() != "200" || !response.IsSuccess() {
		t.Fatal("returnCode precedence lost")
	}
	for _, state := range []string{"3", "204"} {
		if !(&QueryResponse{State: state}).IsSigned() {
			t.Fatal("signed state unrecognized")
		}
	}
	if !(&QueryResponse{IsCheck: "1"}).IsSigned() || (&QueryResponse{State: "0"}).IsSigned() {
		t.Fatal("signature state incorrect")
	}
	tracks := []TrackData{{Ftime: "invalid"}, {Ftime: "2026-01-02 15:04:05"}, {Ftime: "2026-01-01 15:04:05"}, {Ftime: "2026-01-03 15:04:05"}}
	first, ok := FirstTrackUnix(tracks)
	expected, _ := TrackTimeUnix("2026-01-01 15:04:05")
	if !ok || first != expected {
		t.Fatal("earliest track incorrect")
	}
	last, ok := LatestTrackUnix(tracks)
	expected, _ = TrackTimeUnix("2026-01-03 15:04:05")
	if !ok || last != expected {
		t.Fatal("latest track incorrect")
	}
	if _, ok := FirstTrackUnix(nil); ok {
		t.Fatal("empty tracks accepted")
	}
	if _, ok := LatestTrackUnix([]TrackData{{Ftime: "invalid"}}); ok {
		t.Fatal("invalid tracks accepted")
	}
	if got := truncate("short", 100); got != "short" {
		t.Fatal("short text truncated")
	}
}
