package errors

import (
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestE(t *testing.T) {
	t.Parallel()

	err := E(KindNotFound, "user missing")
	assert.Equal(t, KindNotFound, err.Kind)
	assert.Equal(t, "not_found", err.Code)
	assert.Equal(t, "user missing", err.Message)
}

func TestEf(t *testing.T) {
	t.Parallel()

	err := Ef(KindInvalidArgument, "bad value %d", 42)
	assert.Equal(t, KindInvalidArgument, err.Kind)
	assert.Equal(t, "bad value 42", err.Message)
}

func TestWrap(t *testing.T) {
	t.Parallel()

	inner := stderrors.New("db: no rows")
	err := Wrap(inner, "not_found", "user not found")
	assert.Equal(t, KindNotFound, err.Kind)
	assert.Equal(t, "not_found", err.Code)
	assert.ErrorIs(t, err, inner)
	assert.Equal(t, "user not found: db: no rows", err.Error())
}

func TestWrapf(t *testing.T) {
	t.Parallel()

	inner := stderrors.New("db")
	err := Wrapf(inner, "conflict", "order %d conflict", 7)
	assert.Equal(t, KindConflict, err.Kind)
	assert.Equal(t, "order 7 conflict: db", err.Error())
}

func TestWithDetail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		in           error
		detail       string
		expectedMsg  string
		expectedKind Kind
	}{
		{
			name:         "app error",
			in:           E(KindInvalidArgument, "bad input"),
			detail:       "field=email",
			expectedMsg:  "bad input",
			expectedKind: KindInvalidArgument,
		},
		{
			name:         "plain error",
			in:           stderrors.New("boom"),
			detail:       "trace=abc",
			expectedMsg:  "boom",
			expectedKind: KindInternal,
		},
		{
			name:   "nil error",
			in:     nil,
			detail: "ignored",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := WithDetail(tt.in, tt.detail)
			if tt.in == nil {
				assert.NoError(t, got)
				return
			}
			ae, ok := got.(*AppError)
			assert.True(t, ok)
			assert.Equal(t, tt.detail, ae.Detail)
			assert.Equal(t, tt.expectedMsg, ae.Message)
			assert.Equal(t, tt.expectedKind, ae.Kind)
		})
	}
}

func TestWithDetail_DoesNotMutateOriginal(t *testing.T) {
	t.Parallel()

	original := E(KindInvalidArgument, "bad input")
	_ = WithDetail(original, "field=email")
	assert.Empty(t, original.Detail)
}
