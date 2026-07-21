package auth

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// TestCreateUserReturnsUnderlyingExecError 测试 CreateUser 方法在数据库执行错误时，是否正确返回底层错误。
func TestCreateUserReturnsUnderlyingExecError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()
	mock.ExpectExec("INSERT INTO users").
		WithArgs(sqlmock.AnyArg(), "alice", "Alice", "hashed-password").
		WillReturnError(wantErr)

	store := NewMySQLUserStore(db)

	userID, err := store.CreateUser(t.Context(), "alice", "Alice", "hashed-password")

	require.Empty(t, userID)
	require.ErrorIs(t, err, wantErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestGetPasswordHashReturnsUnderlyingQueryError 测试 GetPasswordHash 方法在数据库查询错误时，是否正确返回底层错误。
func TestGetPasswordHashReturnsUnderlyingQueryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()
	mock.ExpectQuery("SELECT id, password_hash FROM users").
		WithArgs("alice").
		WillReturnError(wantErr)

	store := NewMySQLUserStore(db)

	userID, passwordHash, err := store.GetPasswordHash(t.Context(), "alice")

	require.Empty(t, userID)
	require.Empty(t, passwordHash)
	require.ErrorIs(t, err, wantErr)
	require.NoError(t, mock.ExpectationsWereMet())
}
