// 文件用途：构造快递100表单请求和签名，执行查询并校验 HTTP 与业务响应。

package kd100

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	phoneutil "github.com/anheicang/xinlin/base/phone"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

const maxErrorBodySize = 2048

// Client 保存快递100配置和可注入的 HTTP 客户端，不包含业务数据。
type Client struct {
	customer   string       // 快递100客户标识。
	key        string       // 快递100签名密钥。
	apiURL     string       // 快递100查询接口地址。
	httpClient *http.Client // HTTP 执行器，负责超时、连接与传输策略。
}

// NewClient 创建快递100客户端，默认 HTTP 超时为 30 秒。
func NewClient(customer, key, apiURL string) *Client {
	return NewClientWithHTTPClient(customer, key, apiURL, &http.Client{Timeout: 30 * time.Second})
}

// NewClientWithHTTPClient 创建使用指定 HTTP 客户端的查询器，便于统一超时策略或注入测试传输层。
// httpClient 为 nil 时使用默认 30 秒超时；配置中的首尾空白会被清理。
func NewClientWithHTTPClient(customer, key, apiURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{customer: strings.TrimSpace(customer), key: strings.TrimSpace(key), apiURL: strings.TrimSpace(apiURL), httpClient: httpClient}
}

// Enabled 判断快递客户端是否具有 customer、key 和接口地址；nil 客户端返回 false。
func (c *Client) Enabled() bool {
	return c != nil && c.customer != "" && c.key != "" && c.apiURL != ""
}

// generateSign 按快递100协议计算 param+key+customer 的大写 MD5 签名。
// 该 MD5 仅用于供应商请求协议，不用于存储密码。
func (c *Client) generateSign(param string) string {
	hash := md5.Sum([]byte(param + c.key + c.customer))
	return strings.ToUpper(fmt.Sprintf("%x", hash))
}

// QueryExpress 验证请求、计算签名并提交快递查询，返回轨迹响应。
// 支持 context 取消；业务失败可同时返回响应和错误，输入请求会先复制而不直接修改。
func (c *Client) QueryExpress(ctx context.Context, req *QueryRequest) (*QueryResponse, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("快递100客户端配置不完整")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if req == nil {
		return nil, fmt.Errorf("请求参数不能为空")
	}

	query := *req
	query.Company = strings.ToLower(strings.TrimSpace(query.Company))
	query.Number = strings.TrimSpace(query.Number)
	query.Phone = phoneutil.Normalize(query.Phone)
	query.To = strings.TrimSpace(query.To)
	if query.Company == "" || query.Number == "" {
		return nil, fmt.Errorf("快递公司编码和快递单号不能为空")
	}
	if len(query.Number) < 6 || len(query.Number) > 32 {
		return nil, fmt.Errorf("快递单号长度必须为6至32个字符")
	}
	if query.ResultV2 == "" {
		query.ResultV2 = "4"
	}

	paramBytes, err := json.Marshal(&query)
	if err != nil {
		return nil, fmt.Errorf("序列化快递100请求失败: %w", err)
	}
	param := string(paramBytes)
	form := url.Values{"customer": {c.customer}, "sign": {c.generateSign(param)}, "param": {param}}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("创建快递100请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("User-Agent", "Xinlin-KD100/1.0")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求快递100失败: %w", err)
	}
	defer httpResp.Body.Close()
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取快递100响应失败: %w", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("快递100 HTTP 状态异常: status=%d body=%q", httpResp.StatusCode, truncate(string(body), maxErrorBodySize))
	}

	var result QueryResponse
	if err = json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析快递100响应失败: %w", err)
	}
	if !result.IsSuccess() {
		return &result, fmt.Errorf("快递100查询失败: status=%s message=%s", result.APIStatus(), result.Message)
	}
	logx.WithContext(ctx).Infof("快递100查询成功: com=%s nu=%s state=%s", result.Com, result.Nu, result.State)
	return &result, nil
}

// truncate 按字节限制错误响应文本长度，避免上游大响应撑大错误日志。
func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
