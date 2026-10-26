// 文件用途：声明默认系统错误，供无法识别的异常使用统一业务码。

package baseerror

var (
	// ErrSystem 是默认系统异常，使用业务码 10001 和通用描述。
	ErrSystem = NewDefaultError("服务器错误")
)

var (
	// ErrDefault 复用 ErrSystem，供未知错误转换时返回；应视为只读共享值。
	ErrDefault = ErrSystem
)
