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

// UploadResult 上传结果
type UploadResult struct {
	Key      string // OSS 对象 Key
	URL      string // 访问 URL
	Size     int64  // 文件大小
	Bucket   string // Bucket 名称
	Endpoint string // OSS Endpoint
}

// Uploader 上传服务
// 封装文件上传流程，统一路径规范，屏蔽底层 OSS SDK 细节
type Uploader struct {
	factory *OSSClientFactory
	config  AliyunOssConfig
}

// NewUploader 创建上传服务
func NewUploader(config AliyunOssConfig) *Uploader {
	return &Uploader{
		factory: NewOSSClientFactory(config),
		config:  config,
	}
}

// NewUploaderWithFactory 使用已有工厂创建上传服务
func NewUploaderWithFactory(factory *OSSClientFactory) *Uploader {
	return &Uploader{
		factory: factory,
		config:  factory.GetConfig(),
	}
}

// UploadFile 上传文件（核心接口）
// filePath: 存储路径（如 "delivery_notes/O20260120110900046486/20260126161652.pdf"）
// contentType: 内容类型（如 "application/pdf"）
// data: 文件数据
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

// UploadFileWithSession 使用指定会话上传文件
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

// UploadFileWithExpiration 上传文件并设置过期时间
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

// BuildPath 构建存储路径
// 格式: /{业务模块}/{业务ID}/{日期}/{文件名}
func (u *Uploader) BuildPath(module, businessId, fileName string) string {
	date := time.Now().Format("20060102")
	return path.Join(module, businessId, date, fileName)
}

// BuildPathWithTimestamp 构建带时间戳的存储路径
// 格式: /{业务模块}/{业务ID}/{时间戳}.{扩展名}
func (u *Uploader) BuildPathWithTimestamp(module, businessId, ext string) string {
	timestamp := time.Now().Format("20060102150405")
	fileName := timestamp + "." + strings.TrimPrefix(ext, ".")
	return path.Join(module, businessId, fileName)
}

// buildURL 构建访问 URL
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

// FileExists 检查文件是否存在
func (u *Uploader) FileExists(filePath string) (bool, error) {
	bucket, err := u.factory.GetBucketWithDefaultSession()
	if err != nil {
		return false, fmt.Errorf("failed to get bucket: %w", err)
	}
	return bucket.IsObjectExist(filePath)
}

// DeleteFile 删除文件
func (u *Uploader) DeleteFile(filePath string) error {
	bucket, err := u.factory.GetBucketWithDefaultSession()
	if err != nil {
		return fmt.Errorf("failed to get bucket: %w", err)
	}
	return bucket.DeleteObject(filePath)
}

// GetFileURL 获取文件访问 URL
func (u *Uploader) GetFileURL(filePath string) string {
	return u.buildURL(filePath)
}

// GetSignedURL 获取临时签名 URL（用于私有 Bucket）
func (u *Uploader) GetSignedURL(filePath string, expiredInSec int64) (string, error) {
	bucket, err := u.factory.GetBucketWithDefaultSession()
	if err != nil {
		return "", fmt.Errorf("failed to get bucket: %w", err)
	}
	return bucket.SignURL(filePath, oss.HTTPGet, expiredInSec)
}

// GetFactory 获取工厂实例
func (u *Uploader) GetFactory() *OSSClientFactory {
	return u.factory
}
