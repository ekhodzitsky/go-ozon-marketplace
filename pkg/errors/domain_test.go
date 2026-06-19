package errors

import (
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *AppError
		expected string
	}{
		{
			name:     "plain message",
			err:      &AppError{Message: "user not found"},
			expected: "user not found",
		},
		{
			name:     "wrapped error",
			err:      &AppError{Message: "lookup failed", Err: stderrors.New("conn refused")},
			expected: "lookup failed: conn refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	t.Parallel()

	inner := stderrors.New("inner")
	err := Wrap(inner, "not_found", "not found")

	require.ErrorIs(t, err, inner)
	assert.Equal(t, inner, err.Unwrap())
}

func TestNew_DerivesKindFromCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code         string
		expectedKind Kind
	}{
		{"not_found", KindNotFound},
		{"already_exists", KindAlreadyExists},
		{"invalid_argument", KindInvalidArgument},
		{"permission_denied", KindPermissionDenied},
		{"unknown_code", KindInternal},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			t.Parallel()
			err := New(tt.code, "msg")
			assert.Equal(t, tt.expectedKind, err.Kind)
			assert.Equal(t, tt.code, err.Code)
		})
	}
}
