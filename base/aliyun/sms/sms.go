// 文件用途：定义短信消息参数和可替换的发送接口。

package sms

// SmsMessage 描述一条模板短信，Content 中的键需与阿里云模板占位符对应。
type SmsMessage struct {
	TemplateCode string // 本次发送的短信模板编号。
	Content      map[string]interface{}
	Phone        string // 接收手机号，号码校验由业务调用方负责。
}

// SmsHandler 抽象短信发送能力，方便业务项目注入其他供应商或测试替身。
type SmsHandler interface {
	// SendMessage 发送模板短信，失败时返回错误，具体供应商由实现决定。
	SendMessage(message *SmsMessage) error
}
