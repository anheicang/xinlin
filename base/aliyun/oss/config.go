package oss

import "strings"

// ================== 请求/响应结构 ==================

type AddAliyunOssCatalogue struct {
	Key string `json:"key"`
}

type GetAliyunOss struct {
	Prefix string `json:"prefix"`
	Page
}

type Page struct {
	CurPage  int64 `json:"currentPage"`
	PageSize int64 `json:"pageSize"`
}

type GetAliyunOssResponse struct {
	List  interface{} `json:"list"`
	Total int64       `json:"total"`
}

// ================== 配置结构（符合需求文档 4.1） ==================

// AliyunOssConfig OSS + STS 配置结构
// 配置示例:
//
//	OSS:
//	  StsEndpoint: "cn-hangzhou.aliyuncs.com"
//	  RoleArn: "acs:ram::<account-id>:role/<role-name>"
//	  OssEndpoint: "oss-cn-hangzhou.aliyuncs.com"
//	  Bucket: "your-bucket"
//	  Domain: "https://your-bucket.oss-cn-hangzhou.aliyuncs.com"
//	  Region: "cn-hangzhou"
//	  AccessKeyId: "******"
//	  AccessKeySecret: "******"
type AliyunOssConfig struct {
	// STS 身份服务配置
	StsEndpoint string `json:"stsEndpoint" yaml:"StsEndpoint"` // STS 端点，如 "cn-hangzhou.aliyuncs.com"
	RoleArn     string `json:"roleArn" yaml:"RoleArn"`         // RAM Role ARN
	// OSS 存储服务配置
	OssEndpoint     string `json:"ossEndpoint" yaml:"OssEndpoint"`         // OSS 端点，如 "oss-cn-hangzhou.aliyuncs.com"
	Bucket          string `json:"bucket" yaml:"Bucket"`                   // Bucket 名称
	Domain          string `json:"domain" yaml:"Domain"`                   // 访问域名
	Region          string `json:"region" yaml:"Region"`                   // 区域
	AccessKeyId     string `json:"accessKeyId" yaml:"AccessKeyId"`         // 主账户 AccessKeyId（仅用于换取 STS 临时凭证）
	AccessKeySecret string `json:"accessKeySecret" yaml:"AccessKeySecret"` // 主账户 AccessKeySecret
}

// GetOssEndpoint returns a normalized OSS endpoint without scheme or trailing slash.
// Some callers use this value for display or URL composition.
func (c AliyunOssConfig) GetOssEndpoint() string {
	endpoint := strings.TrimSpace(c.OssEndpoint)
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimSuffix(endpoint, "/")
	return endpoint
}
