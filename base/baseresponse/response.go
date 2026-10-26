package baseresponse

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"github.com/anheicang/xinlin/base/baseerror"
	"github.com/anheicang/xinlin/base/proto"
	"github.com/zeromicro/go-zero/core/logx"
	"net/http"
)

const codeServiceUnavailable = 10001

var (
	serviceUnavailable = "服务器开小差了，请稍后再试"
)

// Deprecated: use FormatResponseWithRequest instead
func FormatResponse(data interface{}, err error, w http.ResponseWriter) {
	if err != nil {
		codeErr, ok := baseerror.FromError(err)
		if ok {
			if httpErr, ok := err.(interface{ HttpStatus() int }); ok {
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
			if httpErr, ok := err.(interface{ HttpStatus() int }); ok {
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

// calculateETag 计算 ETag
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

// Deprecated: use HttpParamErrorWithRequest instead
func HttpParamError(w http.ResponseWriter, err error) {
	logx.Error(err)
	proto.HttpParamError(w, err.Error())
}

func HttpParamErrorWithRequest(w http.ResponseWriter, r *http.Request, err error) {
	logx.WithContext(r.Context()).Error(err)
	proto.HttpParamError(w, err.Error())
}

func HttpUnauthorized(w http.ResponseWriter, err error) {
	logx.Error(err)
	proto.HttpUnauthorized(w, err.Error())
}

func HttpForbidden(w http.ResponseWriter, err error) {
	logx.Error(err)
	if codeErr, ok := baseerror.FromError(err); ok {
		proto.HttpError(w, http.StatusForbidden, codeErr.Code(), codeErr.Desc(), codeErr.Data())
		return
	}
	proto.HttpError(w, http.StatusForbidden, http.StatusForbidden, err.Error(), nil)
}

func httpBizError(w http.ResponseWriter, err *baseerror.CodeError) {
	proto.HttpError(w, http.StatusNotAcceptable, err.Code(), err.Desc(), err.Data())
}

func httpServerError(w http.ResponseWriter) {
	proto.HttpError(w, http.StatusInternalServerError, codeServiceUnavailable, serviceUnavailable, nil)
}
