// 文件用途：保留对外业务错误码区间的起始常量，便于旧项目迁移。

package baseerror

const (
	// OPENCode 保留旧接口对外错误码区间的起始值 60000。
	OPENCode = iota + 60000
)
