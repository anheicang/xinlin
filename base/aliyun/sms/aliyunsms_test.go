// 文件用途：验证短信 SDK 请求初始化、HTTPS、模板参数和业务状态码；验证空消息和无法序列化的模板参数会返回错误。

package sms

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// smsTransport 将短信 SDK 的 HTTP 请求交给测试处理器。
type smsTransport func(*http.Request) (*http.Response, error)

// RoundTrip 将 HTTP 请求交给测试处理函数，不访问真实供应商接口。
func (f smsTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestSendMessageInitializesSDKAndChecksBusinessCode 验证短信 SDK 请求初始化、HTTPS、模板参数和业务状态码。
func TestSendMessageInitializesSDKAndChecksBusinessCode(t *testing.T) {
	for _, code := range []string{"OK", "isv.BUSINESS_LIMIT_CONTROL"} {
		t.Run(code, func(t *testing.T) {
			client, err := NewSmsClient(AliyunSmsConfig{RegionId: "cn-hangzhou", AccessKey: "test-key", AccessSecret: "test-secret", SignName: "test-sign"})
			if err != nil {
				t.Fatal(err)
			}
			client.Client.Domain = "sms.fixture.invalid"
			called := false
			client.Client.SetTransport(smsTransport(func(r *http.Request) (*http.Response, error) {
				called = true
				if r.URL.Scheme != "https" || r.URL.Query().Get("Action") != "SendSms" {
					t.Errorf("invalid SDK request: %s", r.URL.Path)
				}
				if r.URL.Query().Get("PhoneNumbers") != "test-phone" || r.URL.Query().Get("TemplateParam") != `{"code":"123456"}` {
					t.Error("message fields missing")
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"Code":"` + code + `","Message":"PRIVATE_DETAIL"}`))}, nil
			}))
			err = client.SendMessage(&SmsMessage{Phone: "test-phone", TemplateCode: "test-template", Content: map[string]interface{}{"code": "123456"}})
			if !called || (code == "OK") != (err == nil) {
				t.Fatalf("called=%v err=%v", called, err)
			}
			if err != nil && strings.Contains(err.Error(), "PRIVATE_DETAIL") {
				t.Fatal("upstream detail leaked")
			}
			client.Client.SetTransport(smsTransport(func(*http.Request) (*http.Response, error) { return nil, errors.New("transport failed") }))
			if err := client.SendMessage(&SmsMessage{}); err == nil {
				t.Fatal("transport error hidden")
			}
		})
	}
	var nilClient *AliyunSmsClient
	if nilClient.SendMessage(nil) == nil {
		t.Fatal("nil client accepted")
	}
}

// TestInvalidSMSMessageContent 验证空消息和无法序列化的模板参数会返回错误。
func TestInvalidSMSMessageContent(t *testing.T) {
	c, err := NewSmsClient(AliyunSmsConfig{RegionId: "cn-hangzhou", AccessKey: "test", AccessSecret: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SendMessage(nil); err == nil {
		t.Fatal("nil message accepted")
	}
	if err := c.SendMessage(&SmsMessage{Content: map[string]interface{}{"invalid": make(chan int)}}); err == nil {
		t.Fatal("non-serializable content accepted")
	}
}
