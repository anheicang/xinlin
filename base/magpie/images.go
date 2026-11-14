// 文件用途：固定图像生成的请求和返回协议，不混用普通文本对话，不下载图片或写入业务存储。
package magpie

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

// ImageRequest 是 POST /images/generations 的固定参数；未填写模型时由图像网关选择默认模型。
// 图像能力与文本模型列表独立，不自动使用 Config.Model。生成不重试，以免重复计费。
type ImageRequest struct {
	Model          string `json:"model,omitempty"`         // 图像模型，不是普通对话模型。
	Prompt         string `json:"prompt"`                  // 用户的图像描述，不包含公共库业务提示词。
	N              int    `json:"n"`                       // 图片数，零值规范为1，最多4张。
	Size           string `json:"size,omitempty"`          // auto、1024x1024、1536x1024或1024x1536。
	Quality        string `json:"quality,omitempty"`       // auto、low、medium或high。
	Background     string `json:"background,omitempty"`    // auto、opaque或transparent。
	OutputFormat   string `json:"output_format,omitempty"` // png、jpeg或webp。
	ResponseFormat string `json:"response_format"`         // b64_json（默认）或url。
}

// GeneratedImage 固定保留一种图片载荷，供应商修订的提示词和可选MIME类型。
type GeneratedImage struct {
	B64JSON       string `json:"b64_json,omitempty"`       // 纯Base64，不含data:前缀。
	URL           string `json:"url,omitempty"`            // HTTP(S)图片地址；调用方自行负责下载访问策略。
	RevisedPrompt string `json:"revised_prompt,omitempty"` // 网关实际返回的修订提示词。
	MIMEType      string `json:"mime_type,omitempty"`      // 可选，不替代实际解码校验。
}

// ImageResponse 是固定的图像生成结果；Usage保持网关计量原文，不假设文本token口径。
type ImageResponse struct {
	ID      string           `json:"id,omitempty"`    // 网关可选的生成编号。
	Created int64            `json:"created"`         // 生成时间戳。
	Model   string           `json:"model,omitempty"` // 网关可选的实际模型。
	Data    []GeneratedImage `json:"data"`            // 必须包含与请求数量相同的图片。
	Usage   json.RawMessage  `json:"usage,omitempty"` // 可选计量信息，不输出到日志。
}

// GenerateImage 发起专门的非流式生图请求。响应上限由Config.MaxResponseBytes控制。
// 返回URL时只校验语法，不在库中发起下载；失败不返回部分图像，不执行自动重试。
func (c *Client) GenerateImage(ctx context.Context, request ImageRequest) (*ImageResponse, error) {
	request.Prompt = strings.TrimSpace(request.Prompt)
	request.Model = strings.TrimSpace(request.Model)
	if request.Prompt == "" || utf8.RuneCountInString(request.Prompt) > 16000 {
		return nil, fmt.Errorf("magpie: 生图描述需要1至16000个字符")
	}
	if request.N == 0 {
		request.N = 1
	}
	if request.N < 1 || request.N > 4 {
		return nil, fmt.Errorf("magpie: 图片数量必须为1至4")
	}
	if request.ResponseFormat == "" {
		request.ResponseFormat = "b64_json"
	}
	for _, field := range []struct {
		value   string
		allowed []string
	}{
		{request.Size, []string{"", "auto", "1024x1024", "1536x1024", "1024x1536"}},
		{request.Quality, []string{"", "auto", "low", "medium", "high"}},
		{request.Background, []string{"", "auto", "opaque", "transparent"}},
		{request.OutputFormat, []string{"", "png", "jpeg", "webp"}},
		{request.ResponseFormat, []string{"b64_json", "url"}},
	} {
		if !imageOption(field.value, field.allowed) {
			return nil, fmt.Errorf("magpie: 生图选项不受支持")
		}
	}
	if request.Background == "transparent" && request.OutputFormat == "jpeg" {
		return nil, fmt.Errorf("magpie: JPEG不支持透明背景")
	}
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("magpie: 生图请求不能序列化")
	}
	ctx, cancel := c.callContext(ctx)
	defer cancel()
	response, err := c.do(ctx, http.MethodPost, "/images/generations", body, false)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	payload, err := readLimited(response.Body, c.cfg.MaxResponseBytes)
	if err != nil {
		return nil, err
	}
	if err = responseError(response, payload); err != nil {
		return nil, err
	}
	var result ImageResponse
	if json.Unmarshal(payload, &result) != nil || len(result.Data) != request.N {
		return nil, ErrInvalidResponse
	}
	for _, image := range result.Data {
		if (image.B64JSON == "") == (image.URL == "") {
			return nil, ErrInvalidResponse
		}
		if image.B64JSON != "" {
			raw, err := base64.StdEncoding.DecodeString(image.B64JSON)
			if err != nil || len(raw) == 0 {
				return nil, ErrInvalidResponse
			}
		}
		if image.URL != "" {
			u, err := url.Parse(image.URL)
			if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
				return nil, ErrInvalidResponse
			}
		}
	}
	return &result, nil
}

// imageOption 检查固定协议的枚举值，不接受供应商扩展覆盖标准字段。
func imageOption(value string, allowed []string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
