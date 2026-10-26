// 文件用途：定义短信服务凭证、签名及旧业务模板配置字段。

package sms

type (
	// AliyunSmsConfig 定义阿里云短信配置；部分模板字段为旧业务兼容保留。
	AliyunSmsConfig struct {
		RegionId                     string `json:"regionId"`                     // 阿里云服务区域，例如 cn-hangzhou。
		AccessKey                    string `json:"accessKey"`                    // 阿里云访问密钥 ID，由业务配置注入。
		AccessSecret                 string `json:"accessSecret"`                 // 阿里云访问密钥 Secret，禁止提交或输出日志。
		SignName                     string `json:"signName"`                     // 已审核的短信签名名称。
		TemplateCode                 string `json:"templateCode"`                 // 旧配置保留的模板字段；实际发送使用 SmsMessage.TemplateCode。
		ShopAuditSuccessTemplateCode string `json:"ShopAuditSuccessTemplateCode"` // 商户审核成功模板，旧业务兼容保留字段。
		ShopAuditFailTemplateCode    string `json:"ShopAuditFailTemplateCode"`    // 商户审核失败模板，旧业务兼容保留字段。
		Scheme                       string `json:"scheme"`                       // 旧配置保留字段；当前 SendMessage 固定使用 HTTPS。
	}
)
