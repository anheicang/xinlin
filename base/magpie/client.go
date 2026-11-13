// 文件用途：执行通用 Chat Completions、JSON 解码和模型查询，统一鉴权、限长与可取消的重试。
package magpie

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Chat 发送非流式对话，返回完整响应；不把工具调用误判为空回复。
func (c *Client) Chat(ctx context.Context, request ChatRequest) (*ChatResponse, error) {
	body, err := c.encodeRequest(request, false)
	if err != nil {
		return nil, err
	}
	ctx, cancel := c.callContext(ctx)
	defer cancel()
	resp, err := c.do(ctx, http.MethodPost, "/chat/completions", body, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	payload, err := readLimited(resp.Body, c.cfg.MaxResponseBytes)
	if err != nil {
		return nil, err
	}
	if err = responseError(resp, payload); err != nil {
		return nil, err
	}
	var result ChatResponse
	if json.Unmarshal(payload, &result) != nil || len(result.Choices) == 0 {
		return nil, ErrInvalidResponse
	}
	return &result, nil
}

// ChatJSON 默认请求 json_object 并解析首个候选的文本；可显式传 json_schema。
// target 必须为 encoding/json 支持的非 nil 指针。业务校验由调用方完成。
// 返回响应便于检查用量和状态；解码失败时 target 可能部分赋值，调用方不得使用它。
func (c *Client) ChatJSON(ctx context.Context, request ChatRequest, target any) (*ChatResponse, error) {
	if request.ResponseFormat == nil {
		request.ResponseFormat = &ResponseFormat{Type: "json_object"}
	}
	response, err := c.Chat(ctx, request)
	if err != nil {
		return nil, err
	}
	choice := response.Choices[0]
	if choice.FinishReason == "length" {
		return response, ErrTruncated
	}
	content, ok := choice.Message.Content.(string)
	if !ok || strings.TrimSpace(content) == "" {
		return response, ErrNoContent
	}
	if err := json.Unmarshal([]byte(content), target); err != nil {
		// 不嵌入模型原始回复，防止上层直接记录整个业务输入/输出。
		return response, fmt.Errorf("%w: JSON 输出解码失败 (%T)", ErrInvalidResponse, err)
	}
	return response, nil
}

// Models 查询当前可见模型，不要求预先配置 Model。
func (c *Client) Models(ctx context.Context) (*ModelList, error) {
	ctx, cancel := c.callContext(ctx)
	defer cancel()
	resp, err := c.do(ctx, http.MethodGet, "/models", nil, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	payload, err := readLimited(resp.Body, c.cfg.MaxResponseBytes)
	if err != nil {
		return nil, err
	}
	if err = responseError(resp, payload); err != nil {
		return nil, err
	}
	var result ModelList
	if json.Unmarshal(payload, &result) != nil || result.Data == nil {
		return nil, ErrInvalidResponse
	}
	return &result, nil
}

// encodeRequest 复制顶层参数后序列化，禁止扩展参数绕过强类型字段或切换流式模式。
func (c *Client) encodeRequest(request ChatRequest, stream bool) ([]byte, error) {
	request.Model = strings.TrimSpace(request.Model)
	if request.Model == "" {
		request.Model = c.cfg.Model
	}
	if request.Model == "" || len(request.Messages) == 0 {
		return nil, fmt.Errorf("magpie: 模型和消息不能为空")
	}
	for _, message := range request.Messages {
		if strings.TrimSpace(message.Role) == "" {
			return nil, fmt.Errorf("magpie: 消息角色不能为空")
		}
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("magpie: 请求参数不能序列化为 JSON")
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(encoded, &fields) // 上一步产生的合法 JSON 对象，必然能解码。
	for key, value := range request.Extra {
		switch key {
		case "model", "messages", "temperature", "top_p", "max_tokens", "max_completion_tokens", "stop", "response_format", "tools", "tool_choice", "stream":
			return nil, fmt.Errorf("magpie: Extra 不能覆盖标准请求字段")
		}
		data, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("magpie: 扩展参数不能序列化为 JSON")
		}
		fields[key] = data
	}
	fields["stream"] = json.RawMessage(strconv.FormatBool(stream))
	return json.Marshal(fields)
}

// callContext 使一次调用的所有尝试、退避和流式读取共享一个超时预算。
func (c *Client) callContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, time.Duration(c.cfg.TimeoutMs)*time.Millisecond)
}

// do 仅重试明确的暂时性 HTTP 状态；不重试连接错误、解码错误或已开始消费的流。
// POST 重试由调用方显式开启 MaxRetries，网关可能已生成内容并产生费用。
func (c *Client) do(ctx context.Context, method, path string, body []byte, retry bool) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("magpie: 创建请求失败: %w", err)
		}
		req.Header.Set("Accept", "application/json, text/event-stream")
		if method == http.MethodPost {
			req.Header.Set("Content-Type", "application/json")
		}
		if c.cfg.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("magpie: HTTP 请求失败: %w", err)
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}
		payload, readErr := readLimited(resp.Body, c.cfg.MaxResponseBytes)
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		apiErr := responseError(resp, payload)
		var statusErr *APIError
		if !retry || attempt >= c.cfg.MaxRetries || !errors.As(apiErr, &statusErr) || !statusErr.Retryable() {
			return nil, apiErr
		}
		timer := time.NewTimer(c.retryDelay(attempt, statusErr.RetryAfter))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

// retryDelay 优先使用有效 Retry-After（秒或 HTTP 日期）；全部等待限制在 30 秒内。
func (c *Client) retryDelay(attempt int, retryAfter string) time.Duration {
	delay := time.Duration(c.cfg.RetryDelayMs) * time.Millisecond * (1 << attempt)
	if seconds, err := strconv.ParseInt(retryAfter, 10, 64); err == nil && seconds >= 0 {
		if seconds > 30 {
			seconds = 30
		}
		delay = time.Duration(seconds) * time.Second
	} else if date, err := http.ParseTime(retryAfter); err == nil {
		if until := time.Until(date); until > 0 {
			delay = until
		}
	}
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	return delay
}

// readLimited 多读取一个字节以识别真实超限，避免截断后被误当成完整 JSON。
func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, fmt.Errorf("magpie: 读取响应失败: %w", err)
	}
	if int64(len(payload)) > limit {
		return nil, ErrResponseTooLarge
	}
	return payload, nil
}

// responseError 兼容非 JSON HTTP 错误和 HTTP 200 中的 error 信封，不保存完整原始响应。
func responseError(resp *http.Response, payload []byte) error {
	var envelope struct {
		Error *struct {
			Message string          `json:"message"`
			Type    string          `json:"type"`
			Code    json.RawMessage `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(payload, &envelope)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && envelope.Error == nil {
		return nil
	}
	err := &APIError{StatusCode: resp.StatusCode, RequestID: resp.Header.Get("X-Request-ID"), RetryAfter: resp.Header.Get("Retry-After")}
	if envelope.Error != nil {
		err.Message, err.Type, err.Code = envelope.Error.Message, envelope.Error.Type, envelope.Error.Code
	}
	return err
}
