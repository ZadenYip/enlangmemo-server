package sync

import (
	"fmt"

	"github.com/google/uuid"
)

func uuidBytes(id string) ([]byte, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid uuid %q: %w", id, err)
	}
	return parsed[:], nil
}

// nullable SQL 辅助函数
// 主要针对在数据库中使用的可空类型，这样子直接传入指针类型即可，避免了在调用方进行判断当前 protobuf msg 字段是不是 optional 的麻烦
func nullableInt32(v *int32) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableString(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}
