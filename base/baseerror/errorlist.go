package baseerror

var (
	ErrSystem = NewDefaultError("服务器错误")
)

var (
	ErrDefault = ErrSystem
)
