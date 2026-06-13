package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type mockProvider struct {
	sendErr     error
	sendCalled  int
	lastTo      string
	lastSubject string
	lastBody    string
}

func (m *mockProvider) Send(ctx context.Context, to, subject, body string) error {
	m.sendCalled++
	m.lastTo = to
	m.lastSubject = subject
	m.lastBody = body
	if m.sendErr != nil {
		return m.sendErr
	}
	return nil
}

func TestSendEmail_Success(t *testing.T) {
	provider := &mockProvider{}
	uc := NewNotificationUsecase(zap.NewNop(), provider, DefaultCallTimeout, DefaultQueryTimeout)

	err := uc.SendEmail(context.Background(), "user@example.com", "Subject", "Body")
	require.NoError(t, err)
	assert.Equal(t, 1, provider.sendCalled)
	assert.Equal(t, "user@example.com", provider.lastTo)
	assert.Equal(t, "Subject", provider.lastSubject)
	assert.Equal(t, "Body", provider.lastBody)
}

func TestSendEmail_EmailNormalization(t *testing.T) {
	provider := &mockProvider{}
	uc := NewNotificationUsecase(zap.NewNop(), provider, DefaultCallTimeout, DefaultQueryTimeout)

	// mail.ParseAddress accepts addresses with display names and angle addresses.
	err := uc.SendEmail(context.Background(), "User <user@example.com>", "Subject", "Body")
	require.NoError(t, err)
	assert.Equal(t, 1, provider.sendCalled)
}

func TestSendEmail_InvalidEmail(t *testing.T) {
	provider := &mockProvider{}
	uc := NewNotificationUsecase(zap.NewNop(), provider, DefaultCallTimeout, DefaultQueryTimeout)

	err := uc.SendEmail(context.Background(), "not-an-email", "Subject", "Body")
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidArgument))
	assert.Equal(t, 0, provider.sendCalled)
}

func TestSendEmail_EmptyEmail(t *testing.T) {
	provider := &mockProvider{}
	uc := NewNotificationUsecase(zap.NewNop(), provider, DefaultCallTimeout, DefaultQueryTimeout)

	err := uc.SendEmail(context.Background(), "", "Subject", "Body")
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidArgument))
	assert.Equal(t, 0, provider.sendCalled)
}

func TestSendEmail_EmptySubject(t *testing.T) {
	provider := &mockProvider{}
	uc := NewNotificationUsecase(zap.NewNop(), provider, DefaultCallTimeout, DefaultQueryTimeout)

	err := uc.SendEmail(context.Background(), "user@example.com", "", "Body")
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidArgument))
	assert.Equal(t, 0, provider.sendCalled)
}

func TestSendEmail_EmptyBody(t *testing.T) {
	provider := &mockProvider{}
	uc := NewNotificationUsecase(zap.NewNop(), provider, DefaultCallTimeout, DefaultQueryTimeout)

	err := uc.SendEmail(context.Background(), "user@example.com", "Subject", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidArgument))
	assert.Equal(t, 0, provider.sendCalled)
}

func TestSendEmail_ProviderError(t *testing.T) {
	provider := &mockProvider{sendErr: errors.New("smtp down")}
	uc := NewNotificationUsecase(zap.NewNop(), provider, DefaultCallTimeout, DefaultQueryTimeout)

	err := uc.SendEmail(context.Background(), "user@example.com", "Subject", "Body")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "send email")
	assert.Equal(t, 1, provider.sendCalled)
}

func TestSendEmail_ContextCancelled(t *testing.T) {
	provider := &mockProvider{sendErr: context.Canceled}
	uc := NewNotificationUsecase(zap.NewNop(), provider, DefaultCallTimeout, DefaultQueryTimeout)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := uc.SendEmail(ctx, "user@example.com", "Subject", "Body")
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestSendEmail_Timeout(t *testing.T) {
	provider := &mockProvider{
		sendErr: context.DeadlineExceeded,
	}
	uc := NewNotificationUsecase(zap.NewNop(), provider, 1*time.Nanosecond, DefaultQueryTimeout)

	err := uc.SendEmail(context.Background(), "user@example.com", "Subject", "Body")
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestNewNotificationUsecase_DefaultTimeouts(t *testing.T) {
	uc := NewNotificationUsecase(zap.NewNop(), &mockProvider{}, 0, 0)
	require.NotNil(t, uc)
}
