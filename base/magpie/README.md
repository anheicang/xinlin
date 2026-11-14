# Magpie 客户端

参考 wowAnalysis 的 Magpie 接入，提供基于标准库的 OpenAI 兼容 `chat/completions` 和 `models` 客户端。提示词、业务结果结构、是否启用 AI、环境变量覆盖和失败降级均由业务项目决定。

本组件尚未发布版本标签，现有 `v0.2.1` 不包含它。发布后在业务模块目录升级依赖；本地联调也可临时执行 `go mod edit -replace=github.com/anheicang/xinlin=E:/project/xinlin`，提交业务依赖前移除该 replace。

## 初始化

```go
import "github.com/anheicang/xinlin/base/magpie"

client, err := magpie.New(magpie.Config{
    BaseURL: "http://127.0.0.1:3425/v1",
    APIKey:  "magpie", // 示例为本机网关默认值，实际值通过业务配置注入。
    Model:   "codex/gpt-6-astra", // 使用网关实际支持的完整模型 ID，不限制供应商。
    TimeoutMs: 120000,
})
if err != nil {
    return err
}
```

也可以直接在 go-zero 配置中声明 `Magpie magpie.Config`：

```yaml
Magpie:
  BaseURL: http://127.0.0.1:3425/v1
  APIKey: magpie
  Model: codex/gpt-6-astra
  TimeoutMs: 120000
  MaxRetries: 0
  RetryDelayMs: 250
  MaxResponseBytes: 2097152
```

所有配置字段均可选；空 BaseURL 使用本机地址，空 APIKey 不发送鉴权头。默认模型可留空用于 `Models` 查询，但对话时必须通过配置或请求指定模型。库不自动读取环境变量，不保存历史消息，不输出日志。需要业务开关时在业务 Config 中另设 `Enabled`，由调用方决定是否发起请求。

客户端可并发复用。通过 `NewWithHTTPClient` 注入已有的代理、TLS、连接池或测试 Transport；传入对象不会被修改，自定义客户端需自行管理重定向策略。默认客户端拒绝重定向，避免请求凭据被转发。

## 普通对话与 JSON 输出

```go
request := magpie.ChatRequest{
    Messages: []magpie.Message{
        {Role: "system", Content: "仅根据输入数据分析，返回包含 summary 字段的 JSON 对象。"},
        {Role: "user", Content: businessDataJSON},
    },
    // 可选：每次覆盖默认模型。
    Model: "group/qiqi",
    // 可选：扩展参数由网关和模型解释。
    Extra: map[string]any{"reasoning_effort": "high"},
}

var result struct {
    Summary string `json:"summary"`
}
response, err := client.ChatJSON(ctx, request, &result)
if err != nil {
    return err
}
// 此处继续校验 summary 是否符合业务要求，再使用 result。
// response.Usage 可用于统计 token；有些网关不返回此字段，需要判断 nil。
_ = response
```

`Chat` 返回完整候选、停止原因和用量；`ChatJSON` 默认添加 `json_object`，也接受显式的 `ResponseFormat{Type: "json_schema", JSONSchema: ...}`。提示词仍需要求输出所需 JSON。库不剥离 Markdown、不修复模型输出，也不把 JSON 语法正确等同于业务有效。JSON 解码失败时目标对象可能部分赋值，不得继续使用；`length` 截断返回 `ErrTruncated`。

`Temperature`、`TopP` 和 token 上限使用指针区分“省略”与“显式传 0”。例如 `temperature := 0.0; request.Temperature = &temperature`。各参数及 JSON Schema 的支持情况取决于网关和模型。

## 流式输出

```go
err := client.Stream(ctx, request, func(chunk magpie.ChatChunk) error {
    for _, choice := range chunk.Choices {
        if text, ok := choice.Delta.Content.(string); ok {
            // 示例：将本次文本增量交给业务层，按 choice.Index 分别累积。
            fmt.Print(text)
        }
    }
    // 返回错误可立即结束接收；业务方也可取消 ctx。
    return nil
})
```

回调同步执行，客户端不创建后台工作协程，也不累积整段文本。回调应及时返回；context 无法中断业务回调本身。推理增量位于 `Delta.ReasoningContent`，工具增量位于 `Delta.ToolCalls`；工具参数片段需按候选序号、工具 Index 拼接。独立用量事件可能没有 Choices，可通过 `Extra["stream_options"] = map[string]any{"include_usage": true}` 请求用量（需网关支持）。

只有收到 SSE `[DONE]` 事件才视为正常完成；提前 EOF 返回 `ErrIncompleteStream`，已显示的部分文本不能当作完整结果。流式调用从不自动重试。

## 工具、多模态与模型发现

- `Message.Content` 可传字符串或兼容协议的多模态内容数组，例如 `[]map[string]any` 中的 `text`、`image_url`。实际图片/音频支持由模型决定。
- `ChatRequest.Tools` 使用 `Tool` / `FunctionDefinition` 描述函数和参数 Schema；`ToolChoice` 可传 `auto`、`none`、`required` 或指定函数对象。
- `ChatResponse.Choices[i].Message.ToolCalls` 返回工具调用；业务方校验并执行工具后，用 `Message{Role: "tool", ToolCallID: id, Content: resultJSON}` 继续对话。公共库不执行工具。
- `client.Models(ctx)` 查询网关实际可见模型 ID，不缓存、不自动替换模型，也不自动降级到其他模型。
- `Extra` 承载网关扩展参数，如 `reasoning_effort`、`seed`、`stream_options`。不能覆盖已有标准字段或 `stream`；标准字段的支持不依赖 Extra。

## 超时、重试和错误

`TimeoutMs` 是整次网络调用的总预算，覆盖请求、重试等待和流式读取；父 context 可提前取消。自定义 HTTP 客户端的超时也会生效。

默认不重试。显式配置 `MaxRetries` 后，非流式请求仅重试 HTTP 429、500、502、503、504；指数退避遵循有效 `Retry-After`，单次等待最多 30 秒。认证/参数错误、网络错误、HTTP 200 错误信封、解析失败和业务校验失败不重试。POST 重试可能重复计费，业务方按需开启。

使用 `errors.As(err, &apiError)` 取得 `*magpie.APIError`，包含 HTTP 状态、网关请求 ID 和错误字段；默认 `Error()` 不回显错误原文。Message/Code 是供应商原文，使用或记录前按业务规则脱敏。`errors.Is` 可识别 `ErrInvalidResponse`、`ErrResponseTooLarge`、`ErrIncompleteStream`、`ErrNoContent`、`ErrTruncated` 和 context 取消/超时。

`MaxResponseBytes` 默认 2 MiB，限制普通响应体以及 SSE 单行、单事件；不限制合法流的总长度。需要更大响应时可配置上限。所有响应体都会在调用结束后关闭。

## 从 wowAnalysis 提取后的接入方式

1. 在业务 `ServiceContext` 中构造共享 `*magpie.Client`，配置可复用 `magpie.Config`。
2. `Analyze`、`Review` 等业务方法保留现有提示词、输入数据和结果结构，改为调用 `ChatJSON`。
3. summary 必填、诊断数量限制、可比性判断、业务降级等仍在业务层执行。
4. 将原来业务层的重试移除或关闭，避免与公共库重试叠加。

本次仅新增基础库组件，未改动 wowAnalysis 项目。离线测试执行 `go test ./base/magpie`，不访问真实 Magpie，也不产生模型调用费用。

专门的图像生成使用 `GenerateImage`，固定请求/返回及边界见 [生图协议](IMAGES.md)。
