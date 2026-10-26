// 文件用途：显式启用时运行真实阿里云 STS 集成校验；默认跳过，不属于离线单元测试。

package oss

import (
	"os"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
)

// TestGetOssSts 显式启用时运行真实阿里云 STS 集成校验；默认跳过，不属于离线单元测试。
func TestGetOssSts(t *testing.T) {
	if os.Getenv("XINLIN_OSS_STS_INTEGRATION") != "1" {
		t.Skip("设置 XINLIN_OSS_STS_INTEGRATION=1 后调用真实阿里云STS")
	}
	configPath := os.Getenv("XINLIN_OSS_CONFIG_FILE")
	if configPath == "" {
		t.Fatal("请通过 XINLIN_OSS_CONFIG_FILE 指定包含 Oss 配置的本地 YAML 文件")
	}
	var config struct{ Oss AliyunOssConfig }
	if err := conf.Load(configPath, &config); err != nil {
		t.Fatal("加载本地OSS配置失败")
	}
	cfg := config.Oss

	resp, err := GetOssSts(cfg, "test-session")
	if err != nil {
		t.Fatal("GetOssSts调用失败，请检查本地配置与授权")
	}

	if resp == nil || resp.Body == nil {
		t.Fatal("STS响应为空")
	}
	cred := resp.Body.Credentials
	if cred == nil {
		t.Fatal("credentials is nil")
	}

	if cred.AccessKeyId == nil || *cred.AccessKeyId == "" || cred.AccessKeySecret == nil || *cred.AccessKeySecret == "" || cred.SecurityToken == nil || *cred.SecurityToken == "" || cred.Expiration == nil || *cred.Expiration == "" {
		t.Fatal("STS临时凭证字段不完整")
	}
}
