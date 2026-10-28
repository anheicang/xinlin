// 文件用途：定义短信连接配置及可选的通用模板映射；业务别名由调用方决定。

package sms

type (
	// AliyunSmsConfig 包含供应商连接配置、签名和可选的模板映射，不固定任何业务场景。
	// 每次发送的模板通过 SmsMessage.TemplateCode 指定，同一客户端可发送多个模板。
	AliyunSmsConfig struct {
		RegionId     string `json:"regionId"`     // 阿里云服务区域，例如 cn-hangzhou。
		AccessKey    string `json:"accessKey"`    // 阿里云访问密钥 ID，由业务配置注入。
		AccessSecret string `json:"accessSecret"` // 阿里云访问密钥 Secret，禁止提交或输出日志。
		SignName     string `json:"signName"`     // 已审核的短信签名名称；当前客户端各模板共用该签名。
		// Templates 将调用方定义的业务别名映射到供应商模板编号，可省略或为空。
		// 例如 login_code -> SMS_001；发送时由调用方选择并传入 TemplateCode，本库不提供默认业务别名。
		Templates map[string]string `json:"templates,optional"`
	}
)
