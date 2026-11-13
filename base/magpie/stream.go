// 文件用途：按 SSE 事件读取流式回复，保留工具/推理/用量增量并识别异常断流。
package magpie

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"strings"
)

// Stream 逐事件同步调用 receive，回调返回错误立即停止并关闭连接。
// 不创建后台 goroutine，不累积整个回复；回调需及时返回，ctx 不能中断回调自身。
// 流式调用不自动重试，避免重复输出或重复执行工具。收到 [DONE] 才表示成功。
func (c *Client) Stream(ctx context.Context, request ChatRequest, receive func(ChatChunk) error) error {
	if receive == nil {
		return fmt.Errorf("magpie: 流式回调不能为空")
	}
	body, err := c.encodeRequest(request, true)
	if err != nil {
		return err
	}
	ctx, cancel := c.callContext(ctx)
	defer cancel()
	resp, err := c.do(ctx, http.MethodPost, "/chat/completions", body, false)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mediaType != "text/event-stream" {
		payload, err := readLimited(resp.Body, c.cfg.MaxResponseBytes)
		if err != nil {
			return err
		}
		if err := responseError(resp, payload); err != nil {
			return err
		}
		return ErrInvalidResponse
	}
	return c.readStream(resp, receive)
}

// readStream 支持注释心跳、CRLF 和多行 data；只限制单行/单事件，不限制合法长流的总长度。
func (c *Client) readStream(resp *http.Response, receive func(ChatChunk) error) error {
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 4096), int(c.cfg.MaxResponseBytes)+1)
	var data []byte
	for scanner.Scan() {
		line := scanner.Text()
		if int64(len(line)) > c.cfg.MaxResponseBytes {
			return ErrResponseTooLarge
		}
		if line == "" {
			if len(data) == 0 {
				continue
			}
			done, err := dispatchEvent(resp, data, receive)
			if err != nil {
				return err
			}
			if done {
				return nil
			}
			data = data[:0]
			continue
		}
		if line == "data" || strings.HasPrefix(line, "data:") {
			value := strings.TrimPrefix(line, "data")
			value = strings.TrimPrefix(value, ":")
			value = strings.TrimPrefix(value, " ")
			if int64(len(data)+len(value)+1) > c.cfg.MaxResponseBytes {
				return ErrResponseTooLarge
			}
			data = append(data, value...)
			data = append(data, '\n')
		}
	}
	if err := scanner.Err(); err != nil {
		if err == bufio.ErrTooLong {
			return ErrResponseTooLarge
		}
		return fmt.Errorf("magpie: 流式读取失败: %w", err)
	}
	// SSE 事件必须由空行结束；EOF 即使落在合法 JSON 后也不能证明生成已完成。
	return ErrIncompleteStream
}

// dispatchEvent 解码一个完整 data 事件；忽略其他 SSE 字段，不把未知对象当成空增量。
func dispatchEvent(resp *http.Response, data []byte, receive func(ChatChunk) error) (bool, error) {
	if strings.TrimSpace(string(data)) == "[DONE]" {
		return true, nil
	}
	if err := responseError(resp, data); err != nil {
		return false, err
	}
	var chunk ChatChunk
	if json.Unmarshal(data, &chunk) != nil || (chunk.Choices == nil && chunk.Usage == nil) {
		return false, ErrInvalidResponse
	}
	if err := receive(chunk); err != nil {
		return false, err
	}
	return false, nil
}
