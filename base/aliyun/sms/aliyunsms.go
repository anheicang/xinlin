package sms

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/dysmsapi"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

type AliyunSmsClient struct {
	Client    *dysmsapi.Client
	RedisConn *redis.Redis
	Config    *AliyunSmsConfig
}

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
		// Do not expose upstream Message, which may contain submitted values.
		return fmt.Errorf("SMS rejected (code=%s)", response.Code)
	}
	return nil
}
