// Package baseresponse 将业务处理结果转换为统一 HTTP JSON 响应，并记录错误日志。
// 新代码优先使用 FormatResponseWithRequest；成功、业务失败和系统异常分别使用约定的 HTTP 状态。
package baseresponse
