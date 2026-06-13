package email

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user@example.com", "use*@example.com"},
		{"ab@example.com", "**@example.com"},
		{"a@example.com", "*@example.com"},
		{"no-at-sign", "no-*******"},
		{"abc", "***"},
		{"abcd", "abc*"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, MaskEmail(tt.input))
		})
	}
}

func TestBuildMessage(t *testing.T) {
	msg := buildMessage("from@example.com", "to@example.com", "Hello", "World")
	assert.Contains(t, string(msg), "From: from@example.com")
	assert.Contains(t, string(msg), "To: to@example.com")
	assert.Contains(t, string(msg), "Subject: Hello")
	assert.Contains(t, string(msg), "World")
}

func TestLogProvider_Send(t *testing.T) {
	var buf bytes.Buffer
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(&buf),
		zapcore.InfoLevel,
	)
	log := zap.New(core)
	provider := NewLogProvider(log)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := provider.Send(ctx, "user@example.com", "Subject", "Body")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "use*@example.com")
	assert.Contains(t, buf.String(), "Subject")
}

func TestSMTPProvider_Send_ContextCancelled(t *testing.T) {
	provider := NewSMTPProvider("127.0.0.1", 1025, "from@example.com", "", "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := provider.Send(ctx, "to@example.com", "Subject", "Body")
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestSMTPProvider_Send_NoAuth(t *testing.T) {
	provider := NewSMTPProvider("127.0.0.1", 1025, "from@example.com", "", "")
	require.NotNil(t, provider)
	assert.Equal(t, "127.0.0.1:1025", provider.addr)
}

func TestSMTPProvider_Send_WithAuth(t *testing.T) {
	provider := NewSMTPProvider("127.0.0.1", 1025, "from@example.com", "user", "pass")
	require.NotNil(t, provider)
	assert.Equal(t, "127.0.0.1:1025", provider.addr)
}
