package auth

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type execErrorConnector struct {
	err error
}

func (c execErrorConnector) Connect(context.Context) (driver.Conn, error) {
	return execErrorConn{err: c.err}, nil
}

func (c execErrorConnector) Driver() driver.Driver {
	return execErrorDriver{}
}

type execErrorDriver struct{}

func (d execErrorDriver) Open(string) (driver.Conn, error) {
	return execErrorConn{}, nil
}

type execErrorConn struct {
	err error
}

func (c execErrorConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c execErrorConn) Close() error {
	return nil
}

func (c execErrorConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c execErrorConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return nil, c.err
}

// TestCreateUserReturnsUnderlyingExecError 测试 CreateUser 方法在数据库执行错误时，是否正确返回底层错误。
func TestCreateUserReturnsUnderlyingExecError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	db := sql.OpenDB(execErrorConnector{err: wantErr})
	defer func() {
		_ = db.Close()
	}()

	store := NewMySQLUserStore(db)

	userID, err := store.CreateUser(t.Context(), "alice", "Alice", "hashed-password")

	require.Empty(t, userID)
	require.ErrorIs(t, err, wantErr)
}
