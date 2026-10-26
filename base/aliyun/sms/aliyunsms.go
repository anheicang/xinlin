// 文件用途：创建短信客户端并发送模板消息，检查 SDK 和短信业务错误。

package sms

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/dysmsapi"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// AliyunSmsClient 组合短信 SDK 和发送配置；RedisConn 为旧接口保留字段，当前发送流程不使用它。
type AliyunSmsClient struct {
	Client    *dysmsapi.Client // 底层供应商 SDK 客户端。
	RedisConn *redis.Redis     // 复用的 Redis 客户端。
	Config    *AliyunSmsConfig // 短信配置；调用方应视为只读并妥善保管凭证。
}

// NewSmsClient 按区域和调用方凭证初始化阿里云短信 SDK，不立即发送短信。
func NewSmsClient(c AliyunSmsConfig) (*AliyunSmsClient, error) {
	client, err := dysmsapi.NewClientWithAccessKey(c.RegionId, c.AccessKey, c.AccessSecret)
	if err != nil {
		return nil, err
	}

	return &AliyunSmsClient{
		Client: client,
		Config: &c,
	}, nil
}

// SendMessage 发送单条模板短信，并检查 SDK 错误及响应业务码。
// 成功必须返回 Code=OK；模板内容需可 JSON 序列化，本函数不保存或验证验证码。
func (client *AliyunSmsClient) SendMessage(message *SmsMessage) error {
	if client == nil || client.Client == nil || client.Config == nil || message == nil {
		return errors.New("invalid SMS client or message")
	}
	content, err := json.Marshal(message.Content)
	if err != nil {
		logx.Errorf("json encode message error:%v", err)
		return err
	}
	request := dysmsapi.CreateSendSmsRequest()
	request.Scheme = "https"
	request.SignName = client.Config.SignName
	request.PhoneNumbers = message.Phone
	request.TemplateCode = message.TemplateCode
	request.TemplateParam = string(content)
	response, err := client.Client.SendSms(request)
	if err != nil {
		return err
	}
	if response == nil {
		return errors.New("empty SMS response")
	}
	if response.Code != "OK" {
		// 上游 Message 可能回显手机号或模板参数，仅返回业务码而不透传原文。
		return fmt.Errorf("SMS rejected (code=%s)", response.Code)
	}
	return nil
}
