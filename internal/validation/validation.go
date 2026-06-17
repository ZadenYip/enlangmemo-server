package validation

import (
	"fmt"
	"unicode/utf8"
)

// ValidError describes a field validation error.
type ValidError struct {
	FieldName string
	Msg       string
}

func (valid *ValidError) Error() string {
	return valid.Msg
}

// 验证字符串长度是否超过 maxLen，如果超过则返回 validationError
// 这里是字符长度不是字节长度
func ValidMaxChars(fieldName string, value string, maxLen int) error {
	if utf8.RuneCountInString(value) <= maxLen {
		return nil
	}

	msg := fmt.Sprintf("%s must not be longer than %d characters", fieldName, maxLen)
	return &ValidError{FieldName: fieldName, Msg: msg}
}
