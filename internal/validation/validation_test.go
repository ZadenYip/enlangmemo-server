package validation

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaxChars(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		value     string
		maxLen    int
		wantErr   bool
		wantMsg   string
	}{
		{
			name:      "short ascii value",
			fieldName: "name",
			value:     "alice",
			maxLen:    16,
			wantErr:   false,
		},
		{
			name:      "exact max length",
			fieldName: "name",
			value:     "abcdefghijklmnop",
			maxLen:    16,
			wantErr:   false,
		},
		{
			name:      "unicode counts as characters",
			fieldName: "name",
			value:     "你好世界",
			maxLen:    4,
			wantErr:   false,
		},
		{
			name:      "too long",
			fieldName: "name",
			value:     "abcdefghijklmnopq",
			maxLen:    16,
			wantErr:   true,
			wantMsg:   "name must not be longer than 16 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator()
			v.CheckField(MaxChars(tt.value, tt.maxLen), tt.fieldName, tt.wantMsg)
			if tt.wantErr {
				require.False(t, v.Valid())
				require.Equal(t, tt.wantMsg, v.FieldErrors[tt.fieldName])

				return
			}

			require.True(t, v.Valid())
		})
	}
}

func TestValidatorNonFieldErrors(t *testing.T) {
	v := NewValidator()

	v.AddNonFieldError("invalid credentials")

	require.False(t, v.Valid())
	require.Equal(t, []string{"invalid credentials"}, v.NonFieldErrors)
}
