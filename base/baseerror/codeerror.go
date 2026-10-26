package baseerror

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
	switch err.(type) {
	case *CodeError:
		return true
	}
	return false
}

func FromError(err error) (codeErr *CodeError, ok bool) {
	if se, ok := err.(*CodeError); ok {
		return se, true
	}
	if he, ok := err.(*HttpCodeError); ok {
		return he.CodeError, true
	}
	return nil, false
}

func ToCodeError(err error) *CodeError {
	if IsCodeError(err) {
		return err.(*CodeError)
	}
	return ErrDefault
}
