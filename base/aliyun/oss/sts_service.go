package oss

import (
	"fmt"
	"sync"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	sts "github.com/alibabacloud-go/sts-20150401/v2/client"
	"github.com/alibabacloud-go/tea/tea"
)

// ================== STS 服务模块（符合需求文档 5.1） ==================

// STSCredentials STS 临时凭证
type STSCredentials struct {
	AccessKeyId     string    // 临时 AccessKeyId
	AccessKeySecret string    // 临时 AccessKeySecret
	SecurityToken   string    // 安全令牌
	Expiration      time.Time // 过期时间
}

// IsExpired 检查凭证是否过期（提前5分钟刷新）
func (c *STSCredentials) IsExpired() bool {
	return time.Now().Add(5 * time.Minute).After(c.Expiration)
}

// IsValid 检查凭证是否有效
func (c *STSCredentials) IsValid() bool {
	return c.AccessKeyId != "" && c.AccessKeySecret != "" && c.SecurityToken != "" && !c.IsExpired()
}

// STSService STS 服务（支持凭证缓存和自动刷新）
type STSService struct {
	config AliyunOssConfig
	mu     sync.RWMutex

	// 凭证缓存（按 sessionName 缓存）
	credentialsCache map[string]*STSCredentials
}

// NewSTSService 创建 STS 服务实例
func NewSTSService(config AliyunOssConfig) *STSService {
	return &STSService{
		config:           config,
		credentialsCache: make(map[string]*STSCredentials),
	}
}

// GetCredentials 获取 STS 临时凭证（支持缓存和自动刷新）
func (s *STSService) GetCredentials(sessionName string) (*STSCredentials, error) {
	// 先尝试从缓存获取
	s.mu.RLock()
	cached, exists := s.credentialsCache[sessionName]
	s.mu.RUnlock()

	if exists && cached.IsValid() {
		return cached, nil
	}

	// 缓存不存在或已过期，重新获取
	return s.refreshCredentials(sessionName)
}

// refreshCredentials 刷新凭证
func (s *STSService) refreshCredentials(sessionName string) (*STSCredentials, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 双重检查，避免重复刷新
	if cached, exists := s.credentialsCache[sessionName]; exists && cached.IsValid() {
		return cached, nil
	}

	// 调用 STS AssumeRole 接口
	credentials, err := s.assumeRole(sessionName)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	s.credentialsCache[sessionName] = credentials

	return credentials, nil
}

// assumeRole 调用阿里云 STS AssumeRole 接口
func (s *STSService) assumeRole(sessionName string) (*STSCredentials, error) {
	stsEndpoint := s.config.StsEndpoint
	if stsEndpoint == "" {
		return nil, fmt.Errorf("STS endpoint is not configured")
	}

	config := &openapi.Config{
		AccessKeyId:     tea.String(s.config.AccessKeyId),
		AccessKeySecret: tea.String(s.config.AccessKeySecret),
		Endpoint:        tea.String("sts." + stsEndpoint),
	}

	client, err := newSTSClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create STS client: %w", err)
	}

	req := &sts.AssumeRoleRequest{
		RoleArn:         tea.String(s.config.RoleArn),
		RoleSessionName: tea.String(sessionName),
		DurationSeconds: tea.Int64(3600), // 1小时有效期
	}

	resp, err := client.AssumeRole(req)
	if err != nil {
		return nil, fmt.Errorf("failed to assume role: %w", err)
	}

	if resp.Body == nil || resp.Body.Credentials == nil {
		return nil, fmt.Errorf("empty credentials returned from STS")
	}

	cred := resp.Body.Credentials

	// 解析过期时间
	expiration, err := time.Parse(time.RFC3339, tea.StringValue(cred.Expiration))
	if err != nil {
		expiration = time.Now().Add(time.Hour)
	}

	return &STSCredentials{
		AccessKeyId:     tea.StringValue(cred.AccessKeyId),
		AccessKeySecret: tea.StringValue(cred.AccessKeySecret),
		SecurityToken:   tea.StringValue(cred.SecurityToken),
		Expiration:      expiration,
	}, nil
}

// ClearCache 清除所有缓存
func (s *STSService) ClearCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.credentialsCache = make(map[string]*STSCredentials)
}

// ClearCacheBySession 清除指定会话的缓存
func (s *STSService) ClearCacheBySession(sessionName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.credentialsCache, sessionName)
}
