// 文件用途：定义响应 JSON 字段和底层 HTTP 状态写入函数。

package proto

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// response 定义底层 HTTP JSON 协议，成功和错误结果共用同一字段结构。
type response struct {
	Code int64  `json:"code"`           // 业务码，成功响应为 0。
	Desc string `json:"desc,omitempty"` // 对外错误描述，成功时通常省略。
	// Data 映射现有微信订阅模板字段，字段编号必须与选用模板一致。
	// 不同模板需由业务项目按微信规范扩展，不能随意修改编号。
	Data    interface{} `json:"data,omitempty"`    // 成功结果或业务错误附加数据。
	Message interface{} `json:"message,omitempty"` // 兼容响应的错误描述，与 desc 保持一致。
}

// HttpError 写入指定 HTTP 状态与业务码的错误 JSON。
// 历史参数 message 实际作为 data 输出；message 响应字段仍使用 desc 文本。
func HttpError(w http.ResponseWriter, httpCode int, appCode int64, desc string, message interface{}) {
	httpx.WriteJson(w, httpCode, response{
		Code:    appCode,
		Data:    message,
		Message: desc,
		Desc:    desc,
	})
}

// HttpOk 写入 HTTP 200 成功响应，业务码为 0，结果放入 data。
func HttpOk(w http.ResponseWriter, data interface{}) {
	httpx.WriteJson(w, http.StatusOK, response{
		Data: data,
	})
}

// HttpParamError 写入 HTTP 400 和参数错误描述，不进行业务错误链识别。
func HttpParamError(w http.ResponseWriter, desc string) {
	httpx.WriteJson(w, http.StatusBadRequest, response{
		Code: http.StatusBadRequest,
		Desc: desc,
	})
}

// HttpUnauthorized 写入 HTTP 401 和未授权描述，供认证失败时使用。
func HttpUnauthorized(w http.ResponseWriter, desc string) {
	httpx.WriteJson(w, http.StatusUnauthorized, response{
		Code: http.StatusUnauthorized,
		Desc: desc,
	})
}
