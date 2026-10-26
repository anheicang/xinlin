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
	ObjectInfo struct {
		Key          string    // 文件名
		LastModified time.Time // 最后修改时间
		ETag         string    // 文件的 ETag
		Size         int64     // 文件大小
		StorageClass string    // 存储类别
		Url          string    // Url
	}

	// AliyunOssClient 兼容旧客户端
	// Deprecated: 建议使用 Uploader 或 OSSClientFactory
	AliyunOssClient struct {
		Client *oss.Client
	}
)

// GetOssSts 获取 STS 临时凭证（兼容旧接口）
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

// NewAliyunOssClient 创建 OSS 客户端（兼容旧接口，使用主账号凭证）
// Deprecated: 建议使用 NewUploader 或 NewOSSClientFactory（使用 STS 临时凭证更安全）
func NewAliyunOssClient(c AliyunOssConfig) (*AliyunOssClient, error) {
	endpoint := c.GetOssEndpoint()
	client, err := newOSSClient(endpoint, c.AccessKeyId, c.AccessKeySecret)
	if err != nil {
		return nil, err
	}
	return &AliyunOssClient{Client: client}, nil
}

// NewAliyunOssClientWithSTS 使用 STS 临时凭证创建 OSS 客户端（推荐）
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

func (a *AliyunOssClient) GetBucket(bucketName string) (*oss.GetBucketInfoResult, error) {
	// 获取存储空间。
	bucket, err := a.Client.GetBucketInfo(bucketName)
	if err != nil {
		return nil, err
	}
	return &bucket, nil
}

// 创建bucket
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

// 用bytes数组上传图片
// 如果要传文件夹的话就是 path=dir/fileName
func (a *AliyunOssClient) UpdateFileByBytes(bucketName, path string, data []byte) error {
	bucket, err := a.Client.Bucket(bucketName)
	if err != nil {
		return err
	}

	return bucket.PutObject(path, bytes.NewReader(data), oss.ContentType("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"))
}

// 用bytes数组上传文件，并设置过期时间
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

// UploadContent uploads arbitrary content with optional contentType.
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

func (a *AliyunOssClient) FileExist(bucketName, filePath string) (bool, error) {
	bucket, err := a.Client.Bucket(bucketName)
	if err != nil {
		return false, err
	}
	return bucket.IsObjectExist(filePath)
}

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

func (a *AliyunOssClient) DownloadToFile(bucketName, filePath, localPath string) error {
	bucket, err := a.Client.Bucket(bucketName)
	if err != nil {
		return err
	}
	return bucket.GetObjectToFile(filePath, localPath)
}

func (a *AliyunOssClient) UpdateFileByPath(bucketName, filePath, ossFilePath string) error {
	data, err := readFileAsBytes(filePath)
	if err != nil {
		return err
	}
	return a.UpdateFileByBytes(bucketName, ossFilePath, data)
}

func readFileAsBytes(filePath string) ([]byte, error) {
	// 读取文件内容
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return data, nil
}
