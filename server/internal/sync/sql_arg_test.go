package sync

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUUIDBytes(t *testing.T) {
	// 成功
	id := "550e8400-e29b-41d4-a716-446655440000"
	bytes, err := uuidBytes(id)
	require.NoError(t, err)
	require.Equal(t, 16, len(bytes))

	parsedUUID, err := uuid.FromBytes(bytes)
	require.NoError(t, err)
	require.Equal(t, id, parsedUUID.String())

	// 失败
	failID := "invalid-uuid"
	_, err = uuidBytes(failID)
	require.Error(t, err)
}

func TestNullableInt32(t *testing.T) {
	// 解析 nil
	var v *int32
	require.Nil(t, nullableInt32(v))

	// 解析实际带有值的指针
	val := int32(42)
	v = &val
	require.Equal(t, int32(42), nullableInt32(v))
}

func TestNullableInt64(t *testing.T) {
	// 解析 nil
	var v *int64
	require.Nil(t, nullableInt64(v))

	// 解析实际带有值的指针
	val := int64(42)
	v = &val
	require.Equal(t, int64(42), nullableInt64(v))
}

func TestNullableString(t *testing.T) {
	// 解析 nil
	var v *string
	require.Nil(t, nullableString(v))

	// 解析实际带有值的指针
	val := "hello"
	v = &val
	require.Equal(t, "hello", nullableString(v))
}
