package validation

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidMaxChars(t *testing.T) {
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
				err := ValidMaxChars(tt.fieldName, tt.value, tt.maxLen)
				if tt.wantErr {
					require.Error(t, err)

					var validErr *ValidError
					require.True(t, errors.As(err, &validErr))
					require.Equal(t, tt.fieldName, validErr.FieldName)
					require.Equal(t, tt.wantMsg, err.Error())

					return
				}

				require.NoError(t, err)
			})
		}
	}
