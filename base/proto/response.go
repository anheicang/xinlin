package proto

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
)

type response struct {
	Code    int64       `json:"code"`
	Desc    string      `json:"desc,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Message interface{} `json:"message,omitempty"`
}

func HttpError(w http.ResponseWriter, httpCode int, appCode int64, desc string, message interface{}) {
	httpx.WriteJson(w, httpCode, response{
		Code:    appCode,
		Data:    message,
		Message: desc,
		Desc:    desc,
	})
}

func HttpOk(w http.ResponseWriter, data interface{}) {
	httpx.WriteJson(w, http.StatusOK, response{
		Data: data,
	})
}

func HttpParamError(w http.ResponseWriter, desc string) {
	httpx.WriteJson(w, http.StatusBadRequest, response{
		Code: http.StatusBadRequest,
		Desc: desc,
	})
}

func HttpUnauthorized(w http.ResponseWriter, desc string) {
	httpx.WriteJson(w, http.StatusUnauthorized, response{
		Code: http.StatusUnauthorized,
		Desc: desc,
	})
}
