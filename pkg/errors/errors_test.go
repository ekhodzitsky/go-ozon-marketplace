package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestToStatus_CodeMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected codes.Code
	}{
		{"nil", nil, codes.OK},
		{"not_found_sentinel", ErrNotFound, codes.NotFound},
		{"already_exists_sentinel", ErrAlreadyExists, codes.AlreadyExists},
		{"invalid_argument_sentinel", ErrInvalidArgument, codes.InvalidArgument},
		{"invalid_credentials_sentinel", ErrInvalidCredentials, codes.Unauthenticated},
		{"conflict_sentinel", ErrConflict, codes.FailedPrecondition},
		{"insufficient_stock_sentinel", ErrInsufficientStock, codes.FailedPrecondition},
		{"failed_precondition_sentinel", ErrFailedPrecondition, codes.FailedPrecondition},
		{"app_error_not_found", New("not_found", "not found"), codes.NotFound},
		{"app_error_already_exists", New("already_exists", "exists"), codes.AlreadyExists},
		{"app_error_invalid_argument", New("invalid_argument", "bad input"), codes.InvalidArgument},
		{"app_error_invalid_credentials", New("invalid_credentials", "bad creds"), codes.Unauthenticated},
		{"app_error_conflict", New("conflict", "conflict"), codes.FailedPrecondition},
		{"app_error_insufficient_stock", New("insufficient_stock", "no stock"), codes.FailedPrecondition},
		{"app_error_failed_precondition", New("failed_precondition", "precondition"), codes.FailedPrecondition},
		{"app_error_unauthenticated", New("unauthenticated", "unauth"), codes.Unauthenticated},
		{"app_error_permission_denied", New("permission_denied", "denied"), codes.PermissionDenied},
		{"app_error_unavailable", New("unavailable", "down"), codes.Unavailable},
		{"app_error_deadline_exceeded", New("deadline_exceeded", "timeout"), codes.DeadlineExceeded},
		{"app_error_unknown", New("unknown", "unknown"), codes.Internal},
		{"plain_error", errors.New("boom"), codes.Internal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ToStatus(tt.err)
			if tt.expected == codes.OK {
				assert.NoError(t, got)
				return
			}
			require.Error(t, got)
			st, ok := status.FromError(got)
			require.True(t, ok)
			assert.Equal(t, tt.expected, st.Code())
		})
	}
}
