// 文件用途：声明 Magpie 的通用对话、工具、流式增量和模型列表协议，不包含业务提示词。
package magpie

import "encoding/json"

// Message 表示一条对话。Content 可为字符串或多模态内容数组；工具回复使用 ToolCallID。
type Message struct {
	Role             string     `json:"role"`                        // system、developer、user、assistant 或 tool，兼容网关扩展角色。
	Content          any        `json:"content,omitempty"`           // 文本、多模态内容，或工具调用时的 nil。
	Name             string     `json:"name,omitempty"`              // 可选的发言者名称。
	ToolCallID       string     `json:"tool_call_id,omitempty"`      // tool 消息对应的调用编号。
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`        // assistant 请求执行的工具。
	ReasoningContent string     `json:"reasoning_content,omitempty"` // 部分模型返回的推理文本。
}

// Tool 声明模型可调用的函数；公共库只传递协议，不执行任何工具。
type Tool struct {
	Type     string             `json:"type"`     // 函数工具使用 function。
	Function FunctionDefinition `json:"function"` // 名称、说明及 JSON Schema。
}

// FunctionDefinition 定义函数输入；Parameters 可传 map 或 json.RawMessage。
type FunctionDefinition struct {
	Name        string `json:"name"`                  // 业务函数名称。
	Description string `json:"description,omitempty"` // 帮助模型选择工具的说明。
	Parameters  any    `json:"parameters,omitempty"`  // 函数参数的 JSON Schema。
	Strict      *bool  `json:"strict,omitempty"`      // 是否严格遵循 Schema，需模型支持。
}

// ToolCall 为完整工具调用；流式响应中各字符串可能只是片段，需按 Index 拼接。
type ToolCall struct {
	Index    *int         `json:"index,omitempty"` // 流式工具的序号，可为 0。
	ID       string       `json:"id,omitempty"`    // 工具调用 ID。
	Type     string       `json:"type,omitempty"`  // 通常为 function。
	Function FunctionCall `json:"function"`        // 函数名与参数文本。
}

// FunctionCall 保存模型输出的参数字符串；执行工具前业务方必须解析并校验。
type FunctionCall struct {
	Name      string `json:"name,omitempty"`      // 函数名或流式名称片段。
	Arguments string `json:"arguments,omitempty"` // JSON 文本或流式参数片段。
}

// ResponseFormat 控制文本、JSON 对象或 JSON Schema 输出，能力取决于所选模型。
type ResponseFormat struct {
	Type       string `json:"type"`                  // text、json_object 或 json_schema。
	JSONSchema any    `json:"json_schema,omitempty"` // json_schema 模式的 name、schema、strict 等定义。
}

// ChatRequest 是单次对话参数。指针字段可区分未设置与显式传 0；不设置的参数交给网关处理。
type ChatRequest struct {
	Model               string          `json:"model"`                           // 留空时使用 Config.Model，不固定某个供应商或模型。
	Messages            []Message       `json:"messages"`                        // 上下文由调用方维护，不在客户端保存。
	Temperature         *float64        `json:"temperature,omitempty"`           // 随机性，允许显式传 0。
	TopP                *float64        `json:"top_p,omitempty"`                 // 核采样阈值。
	MaxTokens           *int            `json:"max_tokens,omitempty"`            // 兼容传统模型的输出上限。
	MaxCompletionTokens *int            `json:"max_completion_tokens,omitempty"` // 新模型的生成 token 上限。
	Stop                []string        `json:"stop,omitempty"`                  // 可选停止字符串。
	ResponseFormat      *ResponseFormat `json:"response_format,omitempty"`       // 文本或结构化输出。
	Tools               []Tool          `json:"tools,omitempty"`                 // 供模型选择的工具定义。
	ToolChoice          any             `json:"tool_choice,omitempty"`           // auto、none、required 或指定工具对象。
	Extra               map[string]any  `json:"-"`                               // 网关扩展参数，例如 reasoning_effort；禁止覆盖已声明字段和 stream。
}

// ChatResponse 保留完整候选与用量，工具调用、截断等状态由调用方判断。
type ChatResponse struct {
	ID      string   `json:"id"`              // 网关响应 ID。
	Object  string   `json:"object"`          // 响应对象类型。
	Created int64    `json:"created"`         // 创建时间戳（秒）。
	Model   string   `json:"model"`           // 实际使用的模型。
	Choices []Choice `json:"choices"`         // 所有候选结果。
	Usage   *Usage   `json:"usage,omitempty"` // 网关提供的计量信息。
}

// Choice 是一个完整候选，FinishReason 通常为 stop、length 或 tool_calls。
type Choice struct {
	Index        int     `json:"index"`         // 候选序号。
	Message      Message `json:"message"`       // 完整回复或工具调用。
	FinishReason string  `json:"finish_reason"` // 停止原因，不静默丢弃截断状态。
}

// Usage 保存通用 token 计数及网关额外提供的细项。
type Usage struct {
	PromptTokens            int64           `json:"prompt_tokens"`                       // 输入 token。
	CompletionTokens        int64           `json:"completion_tokens"`                   // 输出 token。
	TotalTokens             int64           `json:"total_tokens"`                        // 合计 token。
	PromptTokensDetails     json.RawMessage `json:"prompt_tokens_details,omitempty"`     // 缓存等细项。
	CompletionTokensDetails json.RawMessage `json:"completion_tokens_details,omitempty"` // 推理 token 等细项。
}

// ChatChunk 对应一个 SSE data 事件；最后一个用量事件可能没有 Choices。
type ChatChunk struct {
	ID      string        `json:"id"`              // 流式响应 ID。
	Object  string        `json:"object"`          // 响应类型。
	Created int64         `json:"created"`         // 创建时间戳（秒）。
	Model   string        `json:"model"`           // 实际模型。
	Choices []ChunkChoice `json:"choices"`         // 本次各候选的增量。
	Usage   *Usage        `json:"usage,omitempty"` // 可选结束计量。
}

// ChunkChoice 是单个候选的增量。按 Index 分别累积，不假定每个事件都有文本。
type ChunkChoice struct {
	Index        int     `json:"index"`         // 候选序号。
	Delta        Message `json:"delta"`         // 角色、文本、推理或工具调用的增量。
	FinishReason *string `json:"finish_reason"` // nil 表示尚未结束。
}

// Model 描述网关可用模型；ID 可直接用于 ChatRequest.Model。
type Model struct {
	ID      string `json:"id"`       // 含供应商/分组前缀的完整模型 ID。
	Object  string `json:"object"`   // 对象类型。
	Created int64  `json:"created"`  // 模型创建时间（可能为 0）。
	OwnedBy string `json:"owned_by"` // 模型提供方。
}

// ModelList 是 GET /models 的响应，不缓存，便于发现网关配置更新。
type ModelList struct {
	Object string  `json:"object"` // 列表类型。
	Data   []Model `json:"data"`   // 可用模型。
}
