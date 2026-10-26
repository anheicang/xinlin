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

type Client struct {
	customer   string
	key        string
	apiURL     string
	httpClient *http.Client
}

func NewClient(customer, key, apiURL string) *Client {
	return NewClientWithHTTPClient(customer, key, apiURL, &http.Client{Timeout: 30 * time.Second})
}

func NewClientWithHTTPClient(customer, key, apiURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{customer: strings.TrimSpace(customer), key: strings.TrimSpace(key), apiURL: strings.TrimSpace(apiURL), httpClient: httpClient}
}

func (c *Client) Enabled() bool {
	return c != nil && c.customer != "" && c.key != "" && c.apiURL != ""
}

func (c *Client) generateSign(param string) string {
	hash := md5.Sum([]byte(param + c.key + c.customer))
	return strings.ToUpper(fmt.Sprintf("%x", hash))
}

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

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
