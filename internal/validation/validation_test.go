package validation

import (
	"errors"
	"testing"
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
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				var validErr *ValidError
				if !errors.As(err, &validErr) {
					t.Fatalf("expected *ValidError, got %T", err)
				}

				if validErr.FieldName != tt.fieldName {
					t.Fatalf("FieldName = %q, want %q", validErr.FieldName, tt.fieldName)
				}

				if err.Error() != tt.wantMsg {
					t.Fatalf("error message = %q, want %q", err.Error(), tt.wantMsg)
				}

				return
			}

			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		})
	}
}
