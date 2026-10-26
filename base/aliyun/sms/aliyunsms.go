package sms

import (
	"encoding/json"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/dysmsapi"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

type AliyunSmsClient struct {
	Client    *dysmsapi.Client
	RedisConn *redis.Redis
	Config    *AliyunSmsConfig
}

//request := dysmsapi.CreateSendSmsRequest()
//request.Scheme = "https"
//// 生成随机验证码
////code := strconv.Itoa(rand.Intn(10000))
//code := "123456"
//request.PhoneNumbers = req.Phone
//request.SignName = "专家库系统"
//request.TemplateCode = "SMS_462210002"
//request.TemplateParam = fmt.Sprintf(`{"code":"%s"}`, code)

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
	content, err := json.Marshal(message.Content)
	if err != nil {
		logx.Errorf("json encode message error:%v", err)
		return err
	}
	_, err = client.Client.SendSms(&dysmsapi.SendSmsRequest{
		SignName:      client.Config.SignName,
		PhoneNumbers:  message.Phone,
		TemplateCode:  message.TemplateCode,
		TemplateParam: string(content),
	})
	return err
}
