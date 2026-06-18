package errors

import (
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToGraphQLError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		err            error
		expectedMsg    string
		expectedCode   string
		expectedKey    string
		expectedRetry  bool
		expectedDetail string
	}{
		{
			name:          "kind error",
			err:           E(KindNotFound, "user missing"),
			expectedMsg:   "user missing",
			expectedCode:  "NotFound",
			expectedKey:   "NOT_FOUND",
			expectedRetry: false,
		},
		{
			name:           "error with detail",
			err:            WithDetail(E(KindInvalidArgument, "bad input"), "field=email"),
			expectedMsg:    "bad input",
			expectedCode:   "InvalidArgument",
			expectedKey:    "INVALID_ARGUMENT",
			expectedDetail: "field=email",
		},
		{
			name:          "retryable kind",
			err:           E(KindUnavailable, "service down"),
			expectedMsg:   "service down",
			expectedCode:  "Unavailable",
			expectedKey:   "UNAVAILABLE",
			expectedRetry: true,
		},
		{
			name:          "plain error falls back to internal",
			err:           stderrors.New("boom"),
			expectedMsg:   "internal error",
			expectedCode:  "Internal",
			expectedKey:   "INTERNAL",
			expectedRetry: false,
		},
		{
			name: "nil error",
			err:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ToGraphQLError(tt.err)
			if tt.err == nil {
				assert.Nil(t, got)
				return
			}
			assert.NotNil(t, got)
			assert.Equal(t, tt.expectedMsg, got.Message)
			assert.Equal(t, tt.expectedCode, got.Extensions["code"])
			assert.Equal(t, tt.expectedKey, got.Extensions["publicKey"])
			assert.Equal(t, tt.expectedRetry, got.Extensions["retryable"])
			assert.Equal(t, tt.expectedDetail, got.Extensions["detail"])
		})
	}
}
