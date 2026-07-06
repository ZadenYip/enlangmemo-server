package validation

import (
	"unicode/utf8"

	"github.com/zadenyip/enlangmemo-server/internal/aip"
)

type Validator struct {
	FailMsg string
	// NonFieldErrors []string
	FieldErrors map[string]string
}

func NewValidator() *Validator {
	return &Validator{
		FieldErrors: make(map[string]string),
	}
}

func (v *Validator) AddFieldError(key, msg string) {
	if v.FieldErrors == nil {
		v.FieldErrors = make(map[string]string)
	}

	if _, exists := v.FieldErrors[key]; exists {
		return
	}

	v.FieldErrors[key] = msg
}

func (v *Validator) CheckField(ok bool, key, msg string) {
	if !ok {
		v.AddFieldError(key, msg)
	}
}

// 如果没 field errors，则 Valid() 返回 true，否则返回 false。
func (v *Validator) Valid() bool {
	return len(v.FieldErrors) == 0
	//&& len(v.NonFieldErrors) == 0
}

// 如果 value 包含不超过 maxLen 个字符，则 MaxChars() 返回 true。
func MaxChars(value string, maxLen int) bool {
	return utf8.RuneCountInString(value) <= maxLen
}

func (v *Validator) Detail() *aip.BadRequest {
	details := make([]aip.FieldViolation, 0, len(v.FieldErrors))
	for field, msg := range v.FieldErrors {
		details = append(details, aip.FieldViolation{
			Field:       field,
			Description: msg,
		})
	}

	return &aip.BadRequest{
		BadRequestViolation: details,
	}
}
