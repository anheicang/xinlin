// 文件用途：提供顺序执行错误短路和默认业务错误创建工具。

package baseerror

// Glean 按传入顺序执行操作，遇到第一个错误立即停止并返回。
// 不并发执行，也不会回滚已经成功的操作。
func Glean(fns ...func() error) error {
	for _, fn := range fns {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}

// NewDefaultError 使用默认业务码 10001 创建指定描述的错误。
func NewDefaultError(desc string) *CodeError {
	return NewCodeError(10001, desc)
}
