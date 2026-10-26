// 文件用途：提供旧项目使用的 OSS/STS 兼容接口，包括 Bucket、文件与元信息操作。

package oss

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	sts "github.com/alibabacloud-go/sts-20150401/v2/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// ================== 兼容层（保留旧接口） ==================

type (
	// ObjectInfo 保存兼容接口使用的 OSS 对象摘要，不包含文件内容。
	ObjectInfo struct {
		Key          string    // 文件名
		LastModified time.Time // 最后修改时间
		ETag         string    // 文件的 ETag
		Size         int64     // 文件大小
		StorageClass string    // 存储类别
		Url          string    // Url
	}

	// AliyunOssClient 封装旧项目使用的 OSS SDK 客户端；长期使用 STS 时应通过新工厂刷新凭证。
	//
	// Deprecated: 建议使用 Uploader 或 OSSClientFactory
	AliyunOssClient struct {
		Client *oss.Client // 底层供应商 SDK 客户端。
	}
)

// GetOssSts 调用 STS 获取一次临时凭证，兼容旧项目的 900 秒有效期。
// StsEndpoint 按现有配置约定不带 sts. 前缀；此入口不缓存凭证。
//
// Deprecated: 建议使用 STSService.GetCredentials
func GetOssSts(c AliyunOssConfig, roleSessionName string) (*sts.AssumeRoleResponse, error) {
	config := &openapi.Config{
		AccessKeyId:     tea.String(c.AccessKeyId),
		AccessKeySecret: tea.String(c.AccessKeySecret),
		Endpoint:        tea.String("sts." + c.StsEndpoint),
	}

	client, err := newSTSClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create STS client: %w", err)
	}

	req := &sts.AssumeRoleRequest{
		RoleArn:         tea.String(c.RoleArn),
		RoleSessionName: tea.String(roleSessionName),
		DurationSeconds: tea.Int64(900), // 15分钟
	}

	resp, err := client.AssumeRole(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// NewAliyunOssClient 使用配置中的 AccessKey 创建旧版 OSS 客户端。
// 不会自动使用 STS 或刷新凭证，新项目优先使用 Uploader。
//
// Deprecated: 建议使用 NewUploader 或 NewOSSClientFactory（使用 STS 临时凭证更安全）
func NewAliyunOssClient(c AliyunOssConfig) (*AliyunOssClient, error) {
	endpoint := c.GetOssEndpoint()
	client, err := newOSSClient(endpoint, c.AccessKeyId, c.AccessKeySecret)
	if err != nil {
		return nil, err
	}
	return &AliyunOssClient{Client: client}, nil
}

// NewAliyunOssClientWithSTS 获取一次 STS 凭证并创建旧版 OSS 客户端。
// 返回客户端本身不会自动刷新已过期凭证，长期使用应通过工厂获取新客户端。
func NewAliyunOssClientWithSTS(c AliyunOssConfig, sessionName string) (*AliyunOssClient, error) {
	stsService := NewSTSService(c)
	cred, err := stsService.GetCredentials(sessionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get STS credentials: %w", err)
	}

	endpoint := c.GetOssEndpoint()
	client, err := newOSSClient(
		endpoint,
		cred.AccessKeyId,
		cred.AccessKeySecret,
		oss.SecurityToken(cred.SecurityToken),
	)
	if err != nil {
		return nil, err
	}
	return &AliyunOssClient{Client: client}, nil
}

// GetBucket 查询云端 Bucket 信息；与工厂的 GetBucket 不同，此函数会发起网络请求。
func (a *AliyunOssClient) GetBucket(bucketName string) (*oss.GetBucketInfoResult, error) {
	// 获取存储空间。
	bucket, err := a.Client.GetBucketInfo(bucketName)
	if err != nil {
		return nil, err
	}
	return &bucket, nil
}

// CreateBucket 查询 Bucket 是否存在，不存在时按 options 创建。
// 访问拒绝或其他查询/创建错误会原样返回，不会据此推断 Bucket 不存在。
func (a *AliyunOssClient) CreateBucket(bucketName string, options ...oss.Option) error {
	_, err := a.GetBucket(bucketName)
	var serviceErr oss.ServiceError
	ok := errors.As(err, &serviceErr)
	if ok {
		if serviceErr.Code == "NoSuchBucket" {
			if err = a.Client.CreateBucket(bucketName, options...); err != nil {
				return err
			}
		}
	}
	return err
}

// CreateDir 写入以斜杠结尾的零字节对象，用于模拟 OSS 目录。
// OSS 实际按对象 key 存储数据，并无文件系统目录。
func (a *AliyunOssClient) CreateDir(bucketName, dir string) error {
	bucket, err := a.Client.Bucket(bucketName)
	if err != nil {
		return err
	}
	if !strings.HasSuffix(dir, "/") {
		dir = dir + "/" +
			""
	}
	return bucket.PutObject(dir, bytes.NewReader([]byte{}))
}

// UpdateFileByBytes 把字节内容上传到指定 Bucket 和对象 key。
// 兼容接口固定使用 Excel Content-Type；其他文件类型应使用 UploadContent。
func (a *AliyunOssClient) UpdateFileByBytes(bucketName, path string, data []byte) error {
	bucket, err := a.Client.Bucket(bucketName)
	if err != nil {
		return err
	}

	return bucket.PutObject(path, bytes.NewReader(data), oss.ContentType("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"))
}

// UpdateFileByBytesWithExpiration 上传字节内容并设置 HTTP Expires 元信息，兼容接口固定使用 Excel 类型。
// expirationDays 为相对天数；Expires 是缓存元信息，不会自动删除 OSS 对象。
func (a *AliyunOssClient) UpdateFileByBytesWithExpiration(bucketName, path string, data []byte, expirationDays int) error {
	bucket, err := a.Client.Bucket(bucketName)
	if err != nil {
		return err
	}

	// 设置过期时间
	expirationTime := time.Now().AddDate(0, 0, expirationDays)
	options := []oss.Option{
		oss.ContentType("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"),
		oss.Expires(expirationTime),
	}

	return bucket.PutObject(path, bytes.NewReader(data), options...)
}

// UploadContent 上传字节内容，并在 contentType 非空时显式设置 MIME 类型。
func (a *AliyunOssClient) UploadContent(bucketName, path, contentType string, data []byte) error {
	bucket, err := a.Client.Bucket(bucketName)
	if err != nil {
		return err
	}
	options := []oss.Option{}
	if contentType != "" {
		options = append(options, oss.ContentType(contentType))
	}
	return bucket.PutObject(path, bytes.NewReader(data), options...)
}

// FileExist 检查指定 Bucket 中的对象是否存在，区分不存在与查询失败。
func (a *AliyunOssClient) FileExist(bucketName, filePath string) (bool, error) {
	bucket, err := a.Client.Bucket(bucketName)
	if err != nil {
		return false, err
	}
	return bucket.IsObjectExist(filePath)
}

// GetObjectMeta 读取对象 Content-Length 和去掉引号的 ETag。
// 长度无法解析或远端请求失败时返回错误。
func (a *AliyunOssClient) GetObjectMeta(bucketName, filePath string) (size int64, etag string, err error) {
	bucket, err := a.Client.Bucket(bucketName)
	if err != nil {
		return 0, "", err
	}
	header, err := bucket.GetObjectMeta(filePath)
	if err != nil {
		return 0, "", err
	}
	size, err = strconv.ParseInt(header.Get("Content-Length"), 10, 64)
	if err != nil {
		return 0, "", err
	}
	return size, strings.Trim(header.Get("ETag"), "\""), nil
}

// DownloadToFile 把指定 OSS 对象下载至调用方提供的本地路径；本地文件写入由 SDK 处理。
func (a *AliyunOssClient) DownloadToFile(bucketName, filePath, localPath string) error {
	bucket, err := a.Client.Bucket(bucketName)
	if err != nil {
		return err
	}
	return bucket.GetObjectToFile(filePath, localPath)
}

// UpdateFileByPath 读取本地文件全部内容，再使用兼容上传接口写入 OSS。
// 大文件会占用相应内存；此入口沿用固定 Excel Content-Type。
func (a *AliyunOssClient) UpdateFileByPath(bucketName, filePath, ossFilePath string) error {
	data, err := readFileAsBytes(filePath)
	if err != nil {
		return err
	}
	return a.UpdateFileByBytes(bucketName, ossFilePath, data)
}

// readFileAsBytes 读取本地文件内容供兼容上传使用，文件不存在或不可读时返回错误。
func readFileAsBytes(filePath string) ([]byte, error) {
	// 读取文件内容
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return data, nil
}
