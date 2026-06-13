package validation

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"unicode/utf8"

	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
)

// 验证错误类型
type validError struct {
	fieldName string
	msg       string
}

func (valid *validError) Error() string {
	return valid.msg
}

// 处理验证错误，如果是 validationError 则返回 400 Bad Request，否则返回 500 Internal Server Error
func HandleValidError(w http.ResponseWriter, err error) {
	var validErr *validError
	if errors.As(err, &validErr) {
		httpjson.ResponseError(w, http.StatusBadRequest, "INVALID_ARGUMENT", validErr.msg)
	} else {
		log.Printf("Unexpected error: %v", err)
		const hStatus = http.StatusInternalServerError
		httpjson.ResponseError(w, hStatus, "INTERNAL", http.StatusText(hStatus))
	}
}

// 验证字符串长度是否超过 maxLen，如果超过则返回 validationError
// 这里是字符长度不是字节长度
func ValidMaxChars(fieldName string, value string, maxLen int) error {
	if utf8.RuneCountInString(value) <= maxLen {
		return nil
	}

	msg := fmt.Sprintf("%s must not be longer than %d characters", fieldName, maxLen)
	return &validError{fieldName: fieldName, msg: msg}
}
