package auth

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
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

// TestGetUserProfileReturnsProfile 测试 GetUserProfile 是否根据 userID 返回用户信息。
func TestGetUserProfileReturnsProfile(t *testing.T) {
	userUUID := uuid.MustParse("018f4f6d-7d8b-7cc0-b4d5-74a6e2f2dabc")
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()
	mock.ExpectQuery("SELECT login_id, nickname FROM users").
		WithArgs(userUUID[:]).
		WillReturnRows(sqlmock.NewRows([]string{"login_id", "nickname"}).AddRow("alice", "Alice"))

	store := NewMySQLUserStore(db)

	profile, err := store.GetUserProfile(t.Context(), userUUID.String())

	require.NoError(t, err)
	require.Equal(t, UserProfile{
		UserID:   userUUID.String(),
		LoginID:  "alice",
		Nickname: "Alice",
	}, profile)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestGetUserProfileRejectsInvalidUserID 测试非法 userID 不查询数据库并返回非法 userID 错误。
func TestGetUserProfileRejectsInvalidUserID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	store := NewMySQLUserStore(db)

	profile, err := store.GetUserProfile(t.Context(), "not-a-uuid")

	require.Empty(t, profile)
	require.ErrorIs(t, err, ErrInvalidUserID)
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
