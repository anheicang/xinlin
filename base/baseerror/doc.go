// Package baseerror 提供携带业务码、描述和附加数据的错误，以及支持错误链的识别工具。
// 需要指定 HTTP 状态时使用 HttpCodeError；普通未知错误由响应层转换为统一系统异常。
package baseerror
