package oss

import (
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	sts "github.com/alibabacloud-go/sts-20150401/v2/client"
	ossSDK "github.com/aliyun/aliyun-oss-go-sdk/oss"
)

type stsAssumer interface {
	AssumeRole(*sts.AssumeRoleRequest) (*sts.AssumeRoleResponse, error)
}

// Internal construction seams keep unit tests independent of cloud accounts.
var newSTSClient = func(config *openapi.Config) (stsAssumer, error) {
	return sts.NewClient(config)
}

var newOSSClient = ossSDK.New
