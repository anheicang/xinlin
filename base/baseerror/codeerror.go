package baseerror

import "errors"

type CodeError struct {
	code int64
	desc string
	data interface{}
}

func (err *CodeError) Error() string {
	return err.desc
}

func (err *CodeError) Code() int64 {
	return err.code
}

func (err *CodeError) Desc() string {
	return err.desc
}

func (err *CodeError) Data() interface{} {
	return err.data
}

func NewCodeError(code int64, desc string) *CodeError {
	return &CodeError{
		code: code,
		desc: desc,
	}
}

func NewError(code int64, desc string, data interface{}) *CodeError {
	return &CodeError{
		code: code,
		desc: desc,
		data: data,
	}
}

type HttpCodeError struct {
	*CodeError
	httpStatus int
}

func (err *HttpCodeError) HttpStatus() int {
	return err.httpStatus
}

func NewHttpCodeError(httpStatus int, code int64, desc string) *HttpCodeError {
	return &HttpCodeError{
		CodeError:  NewCodeError(code, desc),
		httpStatus: httpStatus,
	}
}

func IsCodeError(err error) bool {
	_, ok := FromError(err)
	return ok
}

func FromError(err error) (codeErr *CodeError, ok bool) {
	// Both error types expose the same business error interface, including
	// through wrappers; errors.As selects the outermost matching error.
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

func ToCodeError(err error) *CodeError {
	if codeErr, ok := FromError(err); ok {
		return codeErr
	}
	return ErrDefault
}
