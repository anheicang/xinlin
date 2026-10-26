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

type fakeSQLDriver struct{}
type fakeSQLConn struct{}

func (fakeSQLDriver) Open(string) (driver.Conn, error)  { return fakeSQLConn{}, nil }
func (fakeSQLConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not used") }
func (fakeSQLConn) Close() error                        { return nil }
func (fakeSQLConn) Begin() (driver.Tx, error)           { return nil, errors.New("not used") }
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
