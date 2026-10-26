// 文件用途：定义 OSS/STS 配置及兼容请求结构，并规范化 OSS endpoint。

package oss

import "strings"

// ================== 请求/响应结构 ==================

// AddAliyunOssCatalogue 定义旧目录创建接口的对象 key 参数。
type AddAliyunOssCatalogue struct {
	Key string `json:"key"` // 要创建的目录对象 key。
}

// GetAliyunOss 定义旧对象列表查询条件，包含 key 前缀和分页信息。
type GetAliyunOss struct {
	Prefix string `json:"prefix"` // 对象 key 查询前缀。
	Page
}

// Page 保存旧接口的当前页与每页条数，不自动执行分页。
type Page struct {
	CurPage  int64 `json:"currentPage"` // 当前页号，沿用旧接口字段 currentPage。
	PageSize int64 `json:"pageSize"`    // 每页条数。
}

// GetAliyunOssResponse 保存旧列表接口的结果集合与总数。
type GetAliyunOssResponse struct {
	List  interface{} `json:"list"`  // 列表查询返回的业务对象集合。
	Total int64       `json:"total"` // 总记录数，不一定等于当前页条数。
}

// ================== 配置结构（符合需求文档 4.1） ==================

// AliyunOssConfig 配置 OSS 访问及 STS 身份交换，凭证由业务项目安全注入。
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
	AccessKeyId     string `json:"accessKeyId" yaml:"AccessKeyId"`         // 调用身份的 AccessKey ID；用于 STS，兼容直连入口也会用于 OSS。
	AccessKeySecret string `json:"accessKeySecret" yaml:"AccessKeySecret"` // 调用身份的 AccessKey Secret，应由业务配置安全注入。
}

// GetOssEndpoint 移除 endpoint 的首尾空白、http/https 协议和末尾斜杠。
// 用于 OSS SDK 初始化和默认访问域名拼接，不会验证域名是否可访问。
func (c AliyunOssConfig) GetOssEndpoint() string {
	endpoint := strings.TrimSpace(c.OssEndpoint)
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimSuffix(endpoint, "/")
	return endpoint
}
