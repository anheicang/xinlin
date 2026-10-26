package sms

type (
	AliyunSmsConfig struct {
		RegionId                     string `json:"regionId"`
		AccessKey                    string `json:"accessKey"`
		AccessSecret                 string `json:"accessSecret"`
		SignName                     string `json:"signName"`
		TemplateCode                 string `json:"templateCode"`
		ShopAuditSuccessTemplateCode string `json:"ShopAuditSuccessTemplateCode"`
		ShopAuditFailTemplateCode    string `json:"ShopAuditFailTemplateCode"`
		Scheme                       string `json:"scheme"`
	}
)
