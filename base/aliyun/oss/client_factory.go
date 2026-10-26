// 文件用途：将 STS 凭证转换为 OSS SDK 客户端和 Bucket 句柄。

package oss

import (
	"fmt"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// ================== OSS Client 工厂模块（符合需求文档 5.2） ==================

// OSSClientFactory 使用 STS 服务获取有效凭证，再创建 OSS 客户端或 Bucket 句柄。
type OSSClientFactory struct {
	config     AliyunOssConfig // 当前服务使用的配置副本。
	stsService *STSService     // 复用的 STS 凭证缓存服务。
}

// NewOSSClientFactory 创建带独立 STS 凭证缓存的 OSS 工厂，通常在服务初始化时复用。
func NewOSSClientFactory(config AliyunOssConfig) *OSSClientFactory {
	return &OSSClientFactory{
		config:     config,
		stsService: NewSTSService(config),
	}
}

// NewOSSClientFactoryWithSTS 使用已有 STS 服务创建工厂，供多个存储入口共享凭证缓存。
// stsService 必须非 nil，且应与传入配置对应。
func NewOSSClientFactoryWithSTS(config AliyunOssConfig, stsService *STSService) *OSSClientFactory {
	return &OSSClientFactory{
		config:     config,
		stsService: stsService,
	}
}

// GetClient 按 sessionName 获取有效 STS 凭证并创建新的 OSS SDK 客户端。
// 凭证缓存由 STSService 管理，返回客户端本身不会自动更新凭证。
func (f *OSSClientFactory) GetClient(sessionName string) (*oss.Client, error) {
	credentials, err := f.stsService.GetCredentials(sessionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get STS credentials: %w", err)
	}

	return f.createClientWithCredentials(credentials)
}

// GetClientWithDefaultSession 使用 default-session 会话名获取 OSS 客户端，适用于服务端统一上传。
func (f *OSSClientFactory) GetClientWithDefaultSession() (*oss.Client, error) {
	return f.GetClient("default-session")
}

// createClientWithCredentials 把有效 STS 凭证转为带 SecurityToken 的 OSS 客户端。
// endpoint 缺失或 SDK 初始化失败时返回错误。
func (f *OSSClientFactory) createClientWithCredentials(cred *STSCredentials) (*oss.Client, error) {
	ossEndpoint := f.config.GetOssEndpoint()
	if ossEndpoint == "" {
		return nil, fmt.Errorf("OSS endpoint is not configured")
	}

	client, err := newOSSClient(
		ossEndpoint,
		cred.AccessKeyId,
		cred.AccessKeySecret,
		oss.SecurityToken(cred.SecurityToken),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OSS client: %w", err)
	}

	return client, nil
}

// GetBucket 使用指定会话创建配置中 Bucket 的操作句柄。
// 此处主要验证 Bucket 名称；实际读写时才访问 OSS，凭证获取可能调用 STS。
func (f *OSSClientFactory) GetBucket(sessionName string) (*oss.Bucket, error) {
	client, err := f.GetClient(sessionName)
	if err != nil {
		return nil, err
	}

	bucket, err := client.Bucket(f.config.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket: %w", err)
	}

	return bucket, nil
}

// GetBucketWithDefaultSession 使用 default-session 凭证创建配置中 Bucket 的操作句柄。
func (f *OSSClientFactory) GetBucketWithDefaultSession() (*oss.Bucket, error) {
	return f.GetBucket("default-session")
}

// GetSTSService 返回工厂复用的 STS 服务，供调用方主动清理凭证缓存。
func (f *OSSClientFactory) GetSTSService() *STSService {
	return f.stsService
}

// GetConfig 返回工厂配置的值副本。
// 配置含凭证信息，调用方不得将其整体写入日志或公开响应。
func (f *OSSClientFactory) GetConfig() AliyunOssConfig {
	return f.config
}
