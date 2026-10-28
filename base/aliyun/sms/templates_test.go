// 文件用途：验证连接配置不依赖业务模板，同一客户端可按消息使用不同模板。

package sms

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
)

// TestConfigRequiresNoBusinessTemplates 验证只配置凭证、区域和签名即可通过 go-zero 配置加载。
func TestConfigRequiresNoBusinessTemplates(t *testing.T) {
	var config AliyunSmsConfig
	err := conf.LoadFromJsonBytes([]byte(`{"regionId":"cn-hangzhou","accessKey":"test-id","accessSecret":"test-secret","signName":"test-sign"}`), &config)
	if err != nil || config.SignName != "test-sign" {
		t.Fatalf("加载无业务模板的配置失败：%v", err)
	}
}

// TestOneClientSendsMultipleTemplates 验证发送模板来自消息，而不是客户端默认模板或固定业务字段。
func TestOneClientSendsMultipleTemplates(t *testing.T) {
	client, err := NewSmsClient(AliyunSmsConfig{RegionId: "cn-hangzhou", AccessKey: "test-id", AccessSecret: "test-secret", SignName: "test-sign"})
	if err != nil {
		t.Fatal(err)
	}
	client.Client.Domain = "sms.fixture.invalid"
	var templates []string
	client.Client.SetTransport(smsTransport(func(r *http.Request) (*http.Response, error) {
		query := r.URL.Query()
		templates = append(templates, query.Get("TemplateCode"))
		if query.Get("SignName") != "test-sign" || r.URL.Scheme != "https" {
			t.Error("公共签名或 HTTPS 配置未生效")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"Code":"OK"}`))}, nil
	}))
	for _, code := range []string{"SMS_LOGIN", "SMS_NOTICE"} {
		if err := client.SendMessage(&SmsMessage{Phone: "test-phone", TemplateCode: code, Content: map[string]interface{}{"value": "example"}}); err != nil {
			t.Fatal(err)
		}
	}
	if len(templates) != 2 || templates[0] != "SMS_LOGIN" || templates[1] != "SMS_NOTICE" {
		t.Fatalf("发送模板错误：%v", templates)
	}
	for _, message := range []*SmsMessage{{Phone: "test-phone"}, {Phone: "test-phone", TemplateCode: " "}, {TemplateCode: "SMS_LOGIN"}} {
		if err := client.SendMessage(message); err == nil {
			t.Fatal("缺失模板或接收人仍允许发送")
		}
	}
	if len(templates) != 2 {
		t.Fatal("无效消息不应访问供应商接口")
	}
}
