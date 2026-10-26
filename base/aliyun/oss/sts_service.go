// 文件用途：按会话名缓存 STS 临时凭证，提前刷新并抑制重复并发请求。

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

// STSCredentials 保存临时访问密钥、SecurityToken 和过期时间，禁止写入普通日志。
type STSCredentials struct {
	AccessKeyId     string    // 临时 AccessKeyId
	AccessKeySecret string    // 临时 AccessKeySecret
	SecurityToken   string    // 安全令牌
	Expiration      time.Time // 过期时间
}

// IsExpired 判断 STS 凭证是否已进入过期前 5 分钟的刷新窗口。
func (c *STSCredentials) IsExpired() bool {
	return time.Now().Add(5 * time.Minute).After(c.Expiration)
}

// IsValid 判断 STS 凭证关键字段是否完整，且未进入提前刷新窗口。
func (c *STSCredentials) IsValid() bool {
	return c.AccessKeyId != "" && c.AccessKeySecret != "" && c.SecurityToken != "" && !c.IsExpired()
}

// STSService 按会话名缓存 STS 凭证，并通过读写锁协调并发刷新。
type STSService struct {
	config AliyunOssConfig // 当前服务使用的配置副本。
	mu     sync.RWMutex    // 保护凭证缓存的并发读写锁。

	// 凭证缓存（按 sessionName 缓存）
	credentialsCache map[string]*STSCredentials
}

// NewSTSService 创建按 sessionName 缓存临时凭证的服务，不立即调用云端。
func NewSTSService(config AliyunOssConfig) *STSService {
	return &STSService{
		config:           config,
		credentialsCache: make(map[string]*STSCredentials),
	}
}

// GetCredentials 复用有效缓存，缺失或即将过期时同步申请新凭证。
// 服务可被多个 goroutine 复用；调用方应将返回凭证视为只读。
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

// refreshCredentials 在写锁内再次检查缓存并刷新，避免并发请求重复调用 STS。
// 锁覆盖云端调用，因此同一服务实例的凭证刷新会串行执行。
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

// assumeRole 构造 AssumeRole 请求并申请 3600 秒临时凭证。
// 读取供应商过期时间；若该时间无法解析，沿用历史行为以当前时间加一小时作为回退值。
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

// ClearCache 清空此 STS 服务的所有缓存，下次获取时重新请求凭证。
func (s *STSService) ClearCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.credentialsCache = make(map[string]*STSCredentials)
}

// ClearCacheBySession 仅清除指定会话名的 STS 缓存，不影响其他会话。
func (s *STSService) ClearCacheBySession(sessionName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.credentialsCache, sessionName)
}
