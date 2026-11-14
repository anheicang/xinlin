# 专用生图接口

`GenerateImage(context.Context, ImageRequest) (*ImageResponse, error)` 对应 `POST /v1/images/generations`，与Chat/ChatJSON分开。所有参数均为强类型字段，没有可覆盖协议的Extra。只负责协议，不写入业务存储，不下载返回的URL，不保存对话历史。

```go
client, err := magpie.New(magpie.Config{
    BaseURL: "http://127.0.0.1:3425/v1",
    APIKey: "magpie", // 从业务配置注入；此处为本机网关默认值
    TimeoutMs: 360000,
    MaxResponseBytes: 32 << 20, // Base64图像通常远大于普通文字响应
})
result, err := client.GenerateImage(ctx, magpie.ImageRequest{
    Model: "codex/gpt-image-2",
    Prompt: "虚构游戏封面，夜色中的城堡，无文字",
    N: 1,
    Size: "1536x1024",
    Quality: "auto",
    OutputFormat: "png",
    ResponseFormat: "b64_json",
})
```

固定请求：

```json
{"model":"codex/gpt-image-2","prompt":"虚构游戏封面","n":1,"size":"1536x1024","quality":"auto","output_format":"png","response_format":"b64_json"}
```

固定返回：

```json
{"created":123,"model":"codex/gpt-image-2","data":[{"b64_json":"...","revised_prompt":"可选修订描述"}],"usage":{}}
```

返回中的id/model/usage/mime_type/revised_prompt均为可选；每张图只能有b64_json或url一种载荷，图片数量必须与请求一致。Base64会校验编码，实际图片格式、尺寸和内容由业务方解码核验。供应商报错、无图片、混合载荷、数量不符、响应超限均返回错误，不返回部分成功图。

默认n=1，response_format=b64_json；Size支持auto/1024x1024/1536x1024/1024x1536，Quality支持auto/low/medium/high，OutputFormat支持png/jpeg/webp。图片最多4张。模型留空时由图像网关决定，绝不使用文本Config.Model；文本models列表不一定列出图像模型。

**生图永不自动重试**，即使Config.MaxRetries>0也不重试。业务应用应使用幂等任务编号、独立超时和私有存储来控制重复提交，超时不代表供应商未生成。

`go test ./base/magpie` 使用本地httptest，不访问真实模型，不消耗生成额度。2026-11-14在本机Magpie另外进行了单次真实联调，返回codex/gpt-image-2及有效PNG；这不是默认测试的一部分。
