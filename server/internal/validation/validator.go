package validation

import (
	"strings"
	"unicode"
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

// MinChars returns true when value contains at least minLen characters.
func MinChars(value string, minLen int) bool {
	return utf8.RuneCountInString(value) >= minLen
}

// NotBlank returns true when value contains at least one non-whitespace character.
func NotBlank(value string) bool {
	return strings.TrimSpace(value) != ""
}

// ASCIIAlnum returns true when value contains only English letters and digits.
func ASCIIAlnum(value string) bool {
	if value == "" {
		return false
	}

	for _, r := range value {
		if r > unicode.MaxASCII || (!unicode.IsLetter(r) && !unicode.IsDigit(r)) {
			return false
		}
	}

	return true
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
