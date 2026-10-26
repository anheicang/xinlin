// 文件用途：关闭 go-zero 可能包含参数值的 SQL 日志，保留正常业务日志。

package safelog

import "github.com/zeromicro/go-zero/core/stores/sqlx"

// DisableSQLStatements 关闭 go-zero SQL 语句日志，包括慢查询和错误语句中的参数。
// 不会关闭业务日志或数据库执行，建议在初始化数据库前调用。
func DisableSQLStatements() { sqlx.DisableLog() }
