package errors

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
)

func TestRegistry_Completeness(t *testing.T) {
	t.Parallel()

	expected := []Kind{
		KindNotFound,
		KindAlreadyExists,
		KindInvalidArgument,
		KindInvalidCredentials,
		KindPermissionDenied,
		KindConflict,
		KindInsufficientStock,
		KindFailedPrecondition,
		KindUnauthenticated,
		KindUnavailable,
		KindDeadlineExceeded,
		KindInternal,
	}

	for _, kind := range expected {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			entry, ok := EntryFor(kind)
			assert.True(t, ok, "every declared Kind must have a registry entry")
			assert.NotZero(t, entry.GRPCCode)
			assert.NotZero(t, entry.HTTPStatus)
			assert.NotEmpty(t, entry.PublicKey)
		})
	}

	assert.ElementsMatch(t, expected, Kinds())
}

func TestRegistry_Mapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind               Kind
		grpcCode           codes.Code
		httpStatus         int
		publicKey          string
		retryable          bool
	}{
		{KindNotFound, codes.NotFound, http.StatusNotFound, "NOT_FOUND", false},
		{KindAlreadyExists, codes.AlreadyExists, http.StatusConflict, "ALREADY_EXISTS", false},
		{KindInvalidArgument, codes.InvalidArgument, http.StatusBadRequest, "INVALID_ARGUMENT", false},
		{KindInvalidCredentials, codes.Unauthenticated, http.StatusUnauthorized, "INVALID_CREDENTIALS", false},
		{KindPermissionDenied, codes.PermissionDenied, http.StatusForbidden, "PERMISSION_DENIED", false},
		{KindConflict, codes.FailedPrecondition, http.StatusConflict, "CONFLICT", false},
		{KindInsufficientStock, codes.FailedPrecondition, http.StatusConflict, "INSUFFICIENT_STOCK", false},
		{KindFailedPrecondition, codes.FailedPrecondition, http.StatusConflict, "FAILED_PRECONDITION", false},
		{KindUnauthenticated, codes.Unauthenticated, http.StatusUnauthorized, "UNAUTHENTICATED", false},
		{KindUnavailable, codes.Unavailable, http.StatusServiceUnavailable, "UNAVAILABLE", true},
		{KindDeadlineExceeded, codes.DeadlineExceeded, http.StatusGatewayTimeout, "DEADLINE_EXCEEDED", true},
		{KindInternal, codes.Internal, http.StatusInternalServerError, "INTERNAL", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			t.Parallel()
			entry, ok := EntryFor(tt.kind)
			requireTrue(t, ok)
			assert.Equal(t, tt.grpcCode, entry.GRPCCode)
			assert.Equal(t, tt.httpStatus, entry.HTTPStatus)
			assert.Equal(t, tt.publicKey, entry.PublicKey)
			assert.Equal(t, tt.retryable, entry.Retryable)
		})
	}
}

func TestParseKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected Kind
	}{
		{"not_found", KindNotFound},
		{"not-found", KindInternal},
		{"", KindInternal},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, ParseKind(tt.input))
		})
	}
}

func requireTrue(t *testing.T, v bool) {
	t.Helper()
	if !v {
		t.Fatal("expected true")
	}
}
