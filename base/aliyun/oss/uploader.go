// 文件用途：封装上传、删除、访问链接与对象 key 生成，适合作为新项目存储入口。

package oss

import (
	"bytes"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// ================== 上传服务模块（符合需求文档 5.3） ==================

// UploadResult 描述已上传对象的 key、访问链接、字节数及所属存储配置。
type UploadResult struct {
	Key      string // OSS 对象 Key
	URL      string // 访问 URL
	Size     int64  // 文件大小
	Bucket   string // Bucket 名称
	Endpoint string // OSS Endpoint
}

// Uploader 作为统一对象存储入口，复用工厂并封装上传、删除和链接生成。
type Uploader struct {
	factory *OSSClientFactory // 提供有效临时凭证及 Bucket 的客户端工厂。
	config  AliyunOssConfig   // 当前服务使用的配置副本。
}

// NewUploader 创建统一上传服务并配置独立工厂，建议在 ServiceContext 中复用。
func NewUploader(config AliyunOssConfig) *Uploader {
	return &Uploader{
		factory: NewOSSClientFactory(config),
		config:  config,
	}
}

// NewUploaderWithFactory 使用已有工厂创建上传服务，使多个入口共享 STS 缓存。
// factory 必须非 nil。
func NewUploaderWithFactory(factory *OSSClientFactory) *Uploader {
	return &Uploader{
		factory: factory,
		config:  factory.GetConfig(),
	}
}

// UploadFile 通过默认 STS 会话上传内存中的文件内容，并返回 key、URL、大小等结果。
// contentType 为空时交由 SDK 处理；返回 URL 不等同于私有 Bucket 的授权链接。
func (u *Uploader) UploadFile(filePath, contentType string, data []byte) (*UploadResult, error) {
	bucket, err := u.factory.GetBucketWithDefaultSession()
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket: %w", err)
	}

	options := []oss.Option{}
	if contentType != "" {
		options = append(options, oss.ContentType(contentType))
	}

	err = bucket.PutObject(filePath, bytes.NewReader(data), options...)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	url := u.buildURL(filePath)

	return &UploadResult{
		Key:      filePath,
		URL:      url,
		Size:     int64(len(data)),
		Bucket:   u.config.Bucket,
		Endpoint: u.config.GetOssEndpoint(),
	}, nil
}

// UploadFileWithSession 使用指定 STS sessionName 上传文件，供按业务身份拆分凭证缓存的场景使用。
func (u *Uploader) UploadFileWithSession(sessionName, filePath, contentType string, data []byte) (*UploadResult, error) {
	bucket, err := u.factory.GetBucket(sessionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket: %w", err)
	}

	options := []oss.Option{}
	if contentType != "" {
		options = append(options, oss.ContentType(contentType))
	}

	err = bucket.PutObject(filePath, bytes.NewReader(data), options...)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	return &UploadResult{
		Key:      filePath,
		URL:      u.buildURL(filePath),
		Size:     int64(len(data)),
		Bucket:   u.config.Bucket,
		Endpoint: u.config.GetOssEndpoint(),
	}, nil
}

// UploadFileWithExpiration 上传文件并设置 HTTP Expires 元信息，expirationDays 为相对天数。
// Expires 不会删除对象；如需定期删除，应另行配置 OSS 生命周期规则。
func (u *Uploader) UploadFileWithExpiration(filePath, contentType string, data []byte, expirationDays int) (*UploadResult, error) {
	bucket, err := u.factory.GetBucketWithDefaultSession()
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket: %w", err)
	}

	expirationTime := time.Now().AddDate(0, 0, expirationDays)
	options := []oss.Option{oss.Expires(expirationTime)}
	if contentType != "" {
		options = append(options, oss.ContentType(contentType))
	}

	err = bucket.PutObject(filePath, bytes.NewReader(data), options...)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	return &UploadResult{
		Key:      filePath,
		URL:      u.buildURL(filePath),
		Size:     int64(len(data)),
		Bucket:   u.config.Bucket,
		Endpoint: u.config.GetOssEndpoint(),
	}, nil
}

// ================== 路径生成工具（符合需求文档 6.1） ==================

// BuildPath 拼接 module/businessId/当天日期/fileName 对象 key，日期使用进程本地时区。
// path.Join 会清理路径段，调用方应校验来自用户的文件名与业务路径。
func (u *Uploader) BuildPath(module, businessId, fileName string) string {
	date := time.Now().Format("20060102")
	return path.Join(module, businessId, date, fileName)
}

// BuildPathWithTimestamp 生成包含秒级本地时间和扩展名的对象 key。
// 同一业务同一秒可能生成相同 key，需避免冲突时由调用方提供唯一文件名。
func (u *Uploader) BuildPathWithTimestamp(module, businessId, ext string) string {
	timestamp := time.Now().Format("20060102150405")
	fileName := timestamp + "." + strings.TrimPrefix(ext, ".")
	return path.Join(module, businessId, fileName)
}

// buildURL 使用配置 Domain 或默认 Bucket 域名拼接访问地址。
// 不生成访问签名，也不对对象 key 进行 URL 转义。
func (u *Uploader) buildURL(filePath string) string {
	domain := u.config.Domain
	if domain == "" {
		domain = fmt.Sprintf("https://%s.%s", u.config.Bucket, u.config.GetOssEndpoint())
	}
	domain = strings.TrimSuffix(domain, "/")
	filePath = strings.TrimPrefix(filePath, "/")
	return domain + "/" + filePath
}

// ================== 文件操作 ==================

// FileExists 通过默认 STS 会话检查对象是否存在；远端查询失败会返回错误。
func (u *Uploader) FileExists(filePath string) (bool, error) {
	bucket, err := u.factory.GetBucketWithDefaultSession()
	if err != nil {
		return false, fmt.Errorf("failed to get bucket: %w", err)
	}
	return bucket.IsObjectExist(filePath)
}

// DeleteFile 通过默认 STS 会话删除对象，不会同步删除业务数据库中的记录。
func (u *Uploader) DeleteFile(filePath string) error {
	bucket, err := u.factory.GetBucketWithDefaultSession()
	if err != nil {
		return fmt.Errorf("failed to get bucket: %w", err)
	}
	return bucket.DeleteObject(filePath)
}

// GetFileURL 只拼接对象的普通访问链接，不调用 OSS，也不验证对象是否存在。
func (u *Uploader) GetFileURL(filePath string) string {
	return u.buildURL(filePath)
}

// GetSignedURL 生成私有对象的临时 GET 签名链接，expiredInSec 为有效期秒数。
// 实际可访问时间还受 STS 凭证剩余有效期限制。
func (u *Uploader) GetSignedURL(filePath string, expiredInSec int64) (string, error) {
	bucket, err := u.factory.GetBucketWithDefaultSession()
	if err != nil {
		return "", fmt.Errorf("failed to get bucket: %w", err)
	}
	return bucket.SignURL(filePath, oss.HTTPGet, expiredInSec)
}

// GetFactory 返回上传服务所用的工厂，便于共享客户端配置或管理 STS 缓存。
func (u *Uploader) GetFactory() *OSSClientFactory {
	return u.factory
}
