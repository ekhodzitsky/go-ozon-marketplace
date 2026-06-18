package errors

import (
	stderrors "errors"
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
		{"permission_denied_sentinel", ErrPermissionDenied, codes.PermissionDenied},
		{"conflict_sentinel", ErrConflict, codes.FailedPrecondition},
		{"insufficient_stock_sentinel", ErrInsufficientStock, codes.FailedPrecondition},
		{"failed_precondition_sentinel", ErrFailedPrecondition, codes.FailedPrecondition},
		{"unauthenticated_sentinel", ErrUnauthenticated, codes.Unauthenticated},
		{"unavailable_sentinel", ErrUnavailable, codes.Unavailable},
		{"deadline_exceeded_sentinel", ErrDeadlineExceeded, codes.DeadlineExceeded},
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
		{"kind_error_not_found", E(KindNotFound, "missing"), codes.NotFound},
		{"kind_error_unavailable", E(KindUnavailable, "down"), codes.Unavailable},
		{"app_error_unknown", New("unknown", "unknown"), codes.Internal},
		{"plain_error", stderrors.New("boom"), codes.Internal},
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

func TestFromStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		statusCode   codes.Code
		message      string
		expectedKind Kind
	}{
		{"not_found", codes.NotFound, "x", KindNotFound},
		{"already_exists", codes.AlreadyExists, "x", KindAlreadyExists},
		{"invalid_argument", codes.InvalidArgument, "x", KindInvalidArgument},
		{"permission_denied", codes.PermissionDenied, "x", KindPermissionDenied},
		{"failed_precondition", codes.FailedPrecondition, "x", KindFailedPrecondition},
		{"unauthenticated", codes.Unauthenticated, "x", KindUnauthenticated},
		{"unavailable", codes.Unavailable, "x", KindUnavailable},
		{"deadline_exceeded", codes.DeadlineExceeded, "x", KindDeadlineExceeded},
		{"internal", codes.Internal, "x", KindInternal},
		{"unknown", codes.DataLoss, "x", KindInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			st := status.Error(tt.statusCode, tt.message)
			ae := FromStatus(st)
			require.NotNil(t, ae)
			assert.Equal(t, tt.expectedKind, ae.Kind)
			assert.Equal(t, tt.message, ae.Message)
		})
	}
}

func TestFromStatus_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, FromStatus(nil))
}

func TestCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected codes.Code
	}{
		{"nil", nil, codes.OK},
		{"app_error", E(KindNotFound, "x"), codes.NotFound},
		{"plain", stderrors.New("x"), codes.Internal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, Code(tt.err))
		})
	}
}

func TestIsKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		kind     Kind
		expected bool
	}{
		{"exact", E(KindNotFound, "x"), KindNotFound, true},
		{"mismatch", E(KindNotFound, "x"), KindInternal, false},
		{"wrapped", stderrors.New("x"), KindNotFound, false},
		{"nil", nil, KindNotFound, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, IsKind(tt.err, tt.kind))
		})
	}
}

func TestRoundTrip_DomainToStatusToDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		err          error
		expectedKind Kind
	}{
		{"not_found", E(KindNotFound, "missing"), KindNotFound},
		{"permission_denied", E(KindPermissionDenied, "no"), KindPermissionDenied},
		{"unavailable", E(KindUnavailable, "down"), KindUnavailable},
		{"failed_precondition", E(KindFailedPrecondition, "pre"), KindFailedPrecondition},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			st := ToStatus(tt.err)
			require.Error(t, st)
			got := FromStatus(st)
			require.NotNil(t, got)
			assert.Equal(t, tt.expectedKind, got.Kind)
		})
	}
}
