// 文件用途：定义短信发送连接配置；业务模板由调用方管理。

package sms

type (
	// AliyunSmsConfig 只包含供应商连接配置和签名，不依赖任何业务场景或模板。
	// 每次发送的模板通过 SmsMessage.TemplateCode 指定，同一客户端可发送多个模板。
	AliyunSmsConfig struct {
		RegionId     string `json:"regionId"`     // 阿里云服务区域，例如 cn-hangzhou。
		AccessKey    string `json:"accessKey"`    // 阿里云访问密钥 ID，由业务配置注入。
		AccessSecret string `json:"accessSecret"` // 阿里云访问密钥 Secret，禁止提交或输出日志。
		SignName     string `json:"signName"`     // 已审核的短信签名名称；当前客户端各模板共用该签名。
	}
)
