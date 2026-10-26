// 文件用途：定义业务错误和带 HTTP 状态的业务错误，支持从包装错误中提取业务信息。

package baseerror

import "errors"

// CodeError 保存业务错误码、可公开描述和可选附加数据，不直接携带 HTTP 状态。
type CodeError struct {
	code int64       // 业务错误码，与 HTTP 状态分开管理。
	desc string      // 面向调用方的错误描述。
	data interface{} // 可公开的错误附加数据。
}

// Error 返回面向业务的错误描述，实现标准 error 接口。
func (err *CodeError) Error() string {
	return err.desc
}

// Code 返回业务错误码，供统一响应与调用方判断错误类别。
func (err *CodeError) Code() int64 {
	return err.code
}

// Desc 返回错误描述文本，用于响应 desc 和 message 字段。
func (err *CodeError) Desc() string {
	return err.desc
}

// Data 返回业务错误的附加信息，例如字段校验详情；无附加信息时为 nil。
func (err *CodeError) Data() interface{} {
	return err.data
}

// NewCodeError 创建只包含业务码和描述的业务错误。
func NewCodeError(code int64, desc string) *CodeError {
	return &CodeError{
		code: code,
		desc: desc,
	}
}

// NewError 创建携带业务码、描述和附加数据的错误。
// data 会进入统一响应的 data 字段，调用方应仅传入可公开的信息。
func NewError(code int64, desc string, data interface{}) *CodeError {
	return &CodeError{
		code: code,
		desc: desc,
		data: data,
	}
}

// HttpCodeError 在业务错误之外附加 HTTP 状态，供统一响应区分 403、409 等结果。
type HttpCodeError struct {
	*CodeError
	httpStatus int // 该错误要求使用的 HTTP 状态码。
}

// HttpStatus 返回该业务错误指定的 HTTP 状态码。
func (err *HttpCodeError) HttpStatus() int {
	return err.httpStatus
}

// NewHttpCodeError 创建显式指定 HTTP 状态的业务错误，例如 403 或 409。
// httpStatus 与 code 分别表示 HTTP 协议状态和业务错误码。
func NewHttpCodeError(httpStatus int, code int64, desc string) *HttpCodeError {
	return &HttpCodeError{
		CodeError:  NewCodeError(code, desc),
		httpStatus: httpStatus,
	}
}

// IsCodeError 判断错误链中是否包含可识别的业务错误，包括带 HTTP 状态的错误。
func IsCodeError(err error) bool {
	_, ok := FromError(err)
	return ok
}

// FromError 沿标准错误链提取业务错误；未知错误或 nil 返回 false。
// 支持 CodeError、HttpCodeError 和实现 Code/Desc/Data 的自定义错误。
func FromError(err error) (codeErr *CodeError, ok bool) {
	// 两类内置错误都通过此接口暴露业务信息；errors.As 还能识别 %w 包装的错误。
	// 如果错误链有多层匹配，提取最外层的业务错误。
	var business interface {
		error
		Code() int64
		Desc() string
		Data() interface{}
	}
	if errors.As(err, &business) {
		switch value := business.(type) {
		case *CodeError:
			return value, value != nil
		case *HttpCodeError:
			if value == nil {
				return nil, false
			}
			return value.CodeError, value.CodeError != nil
		}
		return NewError(business.Code(), business.Desc(), business.Data()), true
	}
	return nil, false
}

// ToCodeError 把可识别错误转换为业务错误；无法识别时返回共享的 ErrDefault。
func ToCodeError(err error) *CodeError {
	if codeErr, ok := FromError(err); ok {
		return codeErr
	}
	return ErrDefault
}
