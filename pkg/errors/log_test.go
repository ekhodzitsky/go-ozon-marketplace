package errors

import (
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		err             error
		expectedKind    Kind
		expectedCode    string
		expectedRetry   bool
		expectedMessage string
	}{
		{
			name:            "kind error",
			err:             E(KindNotFound, "missing"),
			expectedKind:    KindNotFound,
			expectedCode:    "not_found",
			expectedRetry:   false,
			expectedMessage: "missing",
		},
		{
			name:            "retryable kind",
			err:             E(KindDeadlineExceeded, "timeout"),
			expectedKind:    KindDeadlineExceeded,
			expectedCode:    "deadline_exceeded",
			expectedRetry:   true,
			expectedMessage: "timeout",
		},
		{
			name:            "plain error",
			err:             stderrors.New("boom"),
			expectedKind:    KindInternal,
			expectedCode:    "internal",
			expectedRetry:   false,
			expectedMessage: "internal error",
		},
		{
			name: "nil error",
			err:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fields := Fields(tt.err)
			if tt.err == nil {
				assert.Nil(t, fields)
				return
			}
			require.Len(t, fields, 6)

			m := fieldMap(fields)
			assert.Equal(t, string(tt.expectedKind), m["kind"])
			assert.Equal(t, tt.expectedCode, m["code"])
			assert.Equal(t, tt.expectedRetry, m["retryable"])
			assert.Equal(t, tt.expectedMessage, m["message"])
			assert.NotNil(t, m["error"])
		})
	}
}

func TestFields_IncludesDetail(t *testing.T) {
	t.Parallel()

	err := WithDetail(E(KindInvalidArgument, "bad"), "field=email")
	m := fieldMap(Fields(err))
	assert.Equal(t, "field=email", m["detail"])
}

func fieldMap(fields []zap.Field) map[string]any {
	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fields {
		f.AddTo(enc)
	}
	return enc.Fields
}
