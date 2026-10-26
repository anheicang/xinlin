// 文件用途：按统一协议输出成功或错误响应，并区分业务错误和系统异常。

package baseresponse

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/anheicang/xinlin/base/baseerror"
	"github.com/anheicang/xinlin/base/proto"
	"github.com/zeromicro/go-zero/core/logx"
)

const codeServiceUnavailable = 10001

var (
	serviceUnavailable = "服务器开小差了，请稍后再试"
)

// FormatResponse 把结果写为统一 HTTP JSON 响应，是没有请求上下文的兼容入口。
// 成功返回 200，普通业务错误返回 406，未知错误返回 500；显式 HTTP 错误保留其状态。
//
// Deprecated: use FormatResponseWithRequest instead
func FormatResponse(data interface{}, err error, w http.ResponseWriter) {
	if err != nil {
		codeErr, ok := baseerror.FromError(err)
		if ok {
			var httpErr interface{ HttpStatus() int }
			if errors.As(err, &httpErr) {
				proto.HttpError(w, httpErr.HttpStatus(), codeErr.Code(), codeErr.Desc(), codeErr.Data())
			} else {
				httpBizError(w, codeErr)
			}
		} else {
			httpServerError(w)
		}
		logx.Errorf("%+v", err)
	} else {
		proto.HttpOk(w, data)
	}
}

// FormatResponseWithRequest 把业务结果转换为统一响应，并使用请求上下文记录错误。
// 支持包装的业务错误和 HTTP 状态；未知异常对外仅返回通用提示，r 需非 nil。
func FormatResponseWithRequest(data interface{}, err error, w http.ResponseWriter, r *http.Request) {
	//// 计算 ETag
	//etag := calculateETag(data)
	//// 检查客户端的 ETag 是否匹配
	//if match := r.Header.Get("If-None-Match"); match == etag {
	//	w.WriteHeader(http.StatusNotModified)
	//	return
	//}
	//// 设置 ETag 响应头
	//w.Header().Set("ETag", etag)
	//// 设置其他响应头
	//w.Header().Set("Cache-Control", "public, max-age=60")

	if err != nil {
		codeErr, ok := baseerror.FromError(err)
		if ok {
			var httpErr interface{ HttpStatus() int }
			if errors.As(err, &httpErr) {
				proto.HttpError(w, httpErr.HttpStatus(), codeErr.Code(), codeErr.Desc(), codeErr.Data())
			} else {
				httpBizError(w, codeErr)
			}
		} else {
			httpServerError(w)
		}
		logx.WithContext(r.Context()).Error(err)
	} else {
		proto.HttpOk(w, data)
	}
}

// calculateETag 序列化结果并计算 MD5 摘要，供将来启用 ETag 时复用。
// 当前响应流程未启用 ETag；序列化失败返回空字符串。
func calculateETag(response interface{}) string {
	// 将响应内容序列化为字节流
	data, err := json.Marshal(response)
	if err != nil {
		return ""
	}

	// 使用 MD5 计算哈希值
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// HttpParamError 记录参数错误日志并写入 HTTP 400，是没有请求上下文的兼容入口。
//
// Deprecated: use HttpParamErrorWithRequest instead
func HttpParamError(w http.ResponseWriter, err error) {
	logx.Error(err)
	proto.HttpParamError(w, err.Error())
}

// HttpParamErrorWithRequest 写入 HTTP 400 参数错误，并记录带请求上下文的错误日志。
func HttpParamErrorWithRequest(w http.ResponseWriter, r *http.Request, err error) {
	logx.WithContext(r.Context()).Error(err)
	proto.HttpParamError(w, err.Error())
}

// HttpUnauthorized 记录认证错误日志并写入 HTTP 401；不会在此处解析或撤销 token。
func HttpUnauthorized(w http.ResponseWriter, err error) {
	logx.Error(err)
	proto.HttpUnauthorized(w, err.Error())
}

// HttpForbidden 写入 HTTP 403；业务错误保留业务码与附加数据，普通错误使用描述文本。
func HttpForbidden(w http.ResponseWriter, err error) {
	logx.Error(err)
	if codeErr, ok := baseerror.FromError(err); ok {
		proto.HttpError(w, http.StatusForbidden, codeErr.Code(), codeErr.Desc(), codeErr.Data())
		return
	}
	proto.HttpError(w, http.StatusForbidden, http.StatusForbidden, err.Error(), nil)
}

// httpBizError 把普通业务错误写为 HTTP 406，并保留业务码、描述和附加数据。
func httpBizError(w http.ResponseWriter, err *baseerror.CodeError) {
	proto.HttpError(w, http.StatusNotAcceptable, err.Code(), err.Desc(), err.Data())
}

// httpServerError 写入统一 HTTP 500 系统异常提示，避免把内部错误暴露给客户端。
func httpServerError(w http.ResponseWriter) {
	proto.HttpError(w, http.StatusInternalServerError, codeServiceUnavailable, serviceUnavailable, nil)
}
