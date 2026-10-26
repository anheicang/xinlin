// 文件用途：验证关闭 SQL 日志后仍执行数据库操作，并保留业务错误日志。

package safelog

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// fakeSQLDriver 创建纯内存模拟连接，避免 SQL 日志测试访问真实数据库。
type fakeSQLDriver struct{}

// fakeSQLConn 模拟 SQL 执行、耗时及错误，验证日志开关不会改变数据库行为。
type fakeSQLConn struct{}

// Open 返回模拟数据库连接，不连接真实数据库。
func (fakeSQLDriver) Open(string) (driver.Conn, error) { return fakeSQLConn{}, nil }

// Prepare 拒绝本测试不使用的预编译入口，防止意外绕过模拟执行路径。
func (fakeSQLConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not used") }

// Close 完成测试替身的无副作用资源关闭。
func (fakeSQLConn) Close() error { return nil }

// Begin 拒绝本测试未实现的事务入口。
func (fakeSQLConn) Begin() (driver.Tx, error) { return nil, errors.New("not used") }

// ExecContext 验证 SQL 参数未被日志组件改动，并模拟成功或失败执行。
func (fakeSQLConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if len(args) != 1 || args[0].Value != "PRIVATE_SQL_PARAMETER" {
		return nil, errors.New("parameters changed")
	}
	time.Sleep(2 * time.Millisecond)
	if strings.Contains(query, "fail") {
		return nil, errors.New("fixture query failed")
	}
	return driver.RowsAffected(1), nil
}

var registerFake sync.Once

// TestSQLStatementLoggingDisabledWithoutDisablingBusinessLogs 验证关闭 SQL 日志后仍执行数据库操作，并保留业务错误日志。
func TestSQLStatementLoggingDisabledWithoutDisablingBusinessLogs(t *testing.T) {
	output := capture(t)
	DisableSQLStatements()
	registerFake.Do(func() { sql.Register("safe-log-fixture", fakeSQLDriver{}) })
	db, err := sql.Open("safe-log-fixture", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn := sqlx.NewSqlConnFromDB(db)
	sqlx.SetSlowThreshold(time.Nanosecond)
	defer sqlx.SetSlowThreshold(500 * time.Millisecond)
	result, err := conn.ExecCtx(context.Background(), "INSERT INTO fixture(value) VALUES (?)", "PRIVATE_SQL_PARAMETER")
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		t.Fatal("database write changed")
	}
	if _, err = conn.ExecCtx(context.Background(), "UPDATE fail SET value=?", "PRIVATE_SQL_PARAMETER"); err == nil {
		t.Fatal("database error lost")
	}
	logx.Error("business_error_without_arguments")
	logs := output.String()
	if strings.Contains(logs, "PRIVATE_SQL_PARAMETER") || strings.Contains(logs, "INSERT INTO") || strings.Contains(logs, "UPDATE fail") {
		t.Fatal("SQL statements/parameters logged")
	}
	if !strings.Contains(logs, "business_error_without_arguments") {
		t.Fatal("business logs disabled")
	}
}
