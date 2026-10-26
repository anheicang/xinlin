package oss

import (
	"fmt"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// ================== OSS Client 工厂模块（符合需求文档 5.2） ==================

// OSSClientFactory OSS 客户端工厂
// 使用 STS 临时凭证初始化 OSS Client
type OSSClientFactory struct {
	config     AliyunOssConfig
	stsService *STSService
}

// NewOSSClientFactory 创建 OSS 客户端工厂
func NewOSSClientFactory(config AliyunOssConfig) *OSSClientFactory {
	return &OSSClientFactory{
		config:     config,
		stsService: NewSTSService(config),
	}
}

// NewOSSClientFactoryWithSTS 使用已有的 STS 服务创建工厂
func NewOSSClientFactoryWithSTS(config AliyunOssConfig, stsService *STSService) *OSSClientFactory {
	return &OSSClientFactory{
		config:     config,
		stsService: stsService,
	}
}

// GetClient 获取 OSS 客户端（使用 STS 临时凭证）
func (f *OSSClientFactory) GetClient(sessionName string) (*oss.Client, error) {
	credentials, err := f.stsService.GetCredentials(sessionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get STS credentials: %w", err)
	}

	return f.createClientWithCredentials(credentials)
}

// GetClientWithDefaultSession 使用默认会话获取 OSS 客户端
func (f *OSSClientFactory) GetClientWithDefaultSession() (*oss.Client, error) {
	return f.GetClient("default-session")
}

// createClientWithCredentials 使用临时凭证创建 OSS 客户端
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

// GetBucket 获取 Bucket 对象
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

// GetBucketWithDefaultSession 使用默认会话获取 Bucket
func (f *OSSClientFactory) GetBucketWithDefaultSession() (*oss.Bucket, error) {
	return f.GetBucket("default-session")
}

// GetSTSService 获取 STS 服务实例
func (f *OSSClientFactory) GetSTSService() *STSService {
	return f.stsService
}

// GetConfig 获取配置
func (f *OSSClientFactory) GetConfig() AliyunOssConfig {
	return f.config
}
