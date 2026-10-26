// 文件用途：集中创建云 SDK 客户端，允许单元测试注入离线替身。

package oss

import (
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	sts "github.com/alibabacloud-go/sts-20150401/v2/client"
	ossSDK "github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// stsAssumer 声明本包使用的最小 STS 能力，使单元测试可以替换云端调用。
type stsAssumer interface {
	// AssumeRole 请求角色临时凭证，是本包实际依赖的 STS SDK 能力。
	AssumeRole(*sts.AssumeRoleRequest) (*sts.AssumeRoleResponse, error)
}

// newSTSClient 默认创建真实 STS SDK；单元测试可临时替换，以避免真实云账号和网络依赖。
var newSTSClient = func(config *openapi.Config) (stsAssumer, error) {
	return sts.NewClient(config)
}

// newOSSClient 统一创建 OSS SDK 客户端，便于测试注入离线 HTTP 传输层。
var newOSSClient = ossSDK.New
