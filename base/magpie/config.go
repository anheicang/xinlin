// 文件用途：定义可嵌入业务配置的 Magpie 连接参数，并创建可并发复用的客户端。
package magpie

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config 可直接嵌入 go-zero 配置。零值使用本机网关，不自动读取环境变量或启用业务 AI。
// APIKey 留空表示不发送 Authorization；本机要求默认密钥时显式填写 magpie。
type Config struct {
	BaseURL          string `json:"baseURL,optional"`          // API 根地址，默认 http://127.0.0.1:3425/v1，包含 /v1 前缀。
	APIKey           string `json:"apiKey,optional"`           // 网关密钥，由业务配置注入。
	Model            string `json:"model,optional"`            // 默认模型；也可每次请求单独指定。
	TimeoutMs        int64  `json:"timeoutMs,optional"`        // 整次调用（含重试/流式）的超时毫秒，默认 120000。
	MaxRetries       int    `json:"maxRetries,optional"`       // 初次失败后的重试次数，默认 0，上限 10；重试可能重复计费。
	RetryDelayMs     int64  `json:"retryDelayMs,optional"`     // 指数退避初始毫秒，默认 250，上限 30000。
	MaxResponseBytes int64  `json:"maxResponseBytes,optional"` // 普通响应/流式单事件上限，默认 2 MiB。
}

// Client 封装 HTTP 连接和只读配置，可并发调用；调用方不要并发修改请求内的 map/slice。
type Client struct {
	cfg  Config       // 构造时拷贝并规范化的配置。
	http *http.Client // 共享连接池；默认拒绝重定向，避免密钥被转发。
}

// New 创建默认客户端，不探测网关，也不修改全局 HTTP 设置。
func New(cfg Config) (*Client, error) {
	return NewWithHTTPClient(cfg, &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}})
}

// NewWithHTTPClient 注入代理、TLS、连接池或测试客户端；不修改传入对象。
// 自定义客户端自行负责重定向策略，其 Timeout 与 Config.TimeoutMs 中较短者生效。
func NewWithHTTPClient(cfg Config, client *http.Client) (*Client, error) {
	if client == nil {
		return nil, fmt.Errorf("magpie: HTTP 客户端不能为空")
	}
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://127.0.0.1:3425/v1"
	}
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("magpie: BaseURL 必须为无凭据、查询和片段的 HTTP(S) API 根地址")
	}
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	if strings.ContainsAny(cfg.APIKey, "\r\n") {
		return nil, fmt.Errorf("magpie: APIKey 不能包含换行")
	}
	cfg.Model = strings.TrimSpace(cfg.Model)
	if cfg.TimeoutMs == 0 {
		cfg.TimeoutMs = 120000
	}
	if cfg.RetryDelayMs == 0 {
		cfg.RetryDelayMs = 250
	}
	if cfg.MaxResponseBytes == 0 {
		cfg.MaxResponseBytes = 2 << 20
	}
	if cfg.TimeoutMs < 0 || cfg.TimeoutMs > int64((1<<63-1)/time.Millisecond) || cfg.MaxRetries < 0 || cfg.MaxRetries > 10 || cfg.RetryDelayMs < 0 || cfg.RetryDelayMs > 30000 || cfg.MaxResponseBytes < 1 || cfg.MaxResponseBytes > 1<<30 {
		return nil, fmt.Errorf("magpie: 超时、重试或响应大小配置超出允许范围")
	}
	return &Client{cfg: cfg, http: client}, nil
}
