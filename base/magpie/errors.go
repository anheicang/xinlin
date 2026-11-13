// 文件用途：提供可供 errors.Is/errors.As 判断的错误，避免业务层解析错误文本。
package magpie

import (
	"encoding/json"
	"errors"
	"fmt"
)

var (
	ErrInvalidResponse  = errors.New("magpie: 网关响应格式无效")   // 响应 JSON、候选或 SSE 数据不符合协议。
	ErrResponseTooLarge = errors.New("magpie: 响应超过大小限制")   // 普通响应或单个 SSE 事件超出配置上限。
	ErrIncompleteStream = errors.New("magpie: 流式响应未正常结束")  // 未收到 [DONE] 就断开，不能视为完整结果。
	ErrNoContent        = errors.New("magpie: 首个候选没有文本内容") // 工具调用或非文本回复不能直接解码为业务 JSON。
	ErrTruncated        = errors.New("magpie: 模型输出被截断")    // JSON 输出因 token 上限结束，不返回部分业务结果。
)

// APIError 描述网关 HTTP/协议错误。Message、Code 可能包含供应商原文，请按业务规则脱敏后使用。
type APIError struct {
	StatusCode int             // HTTP 状态码，SSE 内的错误通常仍为 200。
	RequestID  string          // 网关 X-Request-ID，便于定位请求。
	Message    string          // 网关 error.message；不自动写日志或暴露给终端用户。
	Type       string          // 网关 error.type。
	Code       json.RawMessage // 网关 error.code，兼容字符串、数字与 null。
	RetryAfter string          // 原始 Retry-After，用于客户端退避。
}

// Error 默认只包含状态码，避免网关错误原文回显提示词、凭据等信息。
func (e *APIError) Error() string {
	return fmt.Sprintf("magpie: 网关请求失败 (HTTP %d)", e.StatusCode)
}

// Retryable 仅标记常见暂时性 HTTP 故障；认证、参数、响应解析和网络错误均不自动重试。
func (e *APIError) Retryable() bool {
	return e.StatusCode == 429 || e.StatusCode == 500 || e.StatusCode == 502 || e.StatusCode == 503 || e.StatusCode == 504
}
