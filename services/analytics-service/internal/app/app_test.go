package app_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/consumer"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/usecase"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

// fakeConsumerGroup is a minimal sarama.ConsumerGroup test double.
type fakeConsumerGroup struct {
	closed bool
}

func (f *fakeConsumerGroup) Consume(ctx context.Context, topics []string, handler sarama.ConsumerGroupHandler) error {
	<-ctx.Done()
	return nil
}

func (f *fakeConsumerGroup) Errors() <-chan error { return nil }

func (f *fakeConsumerGroup) Close() error {
	f.closed = true
	return nil
}

func (f *fakeConsumerGroup) Pause(partitions map[string][]int32)  {}
func (f *fakeConsumerGroup) Resume(partitions map[string][]int32) {}
func (f *fakeConsumerGroup) PauseAll()                            {}
func (f *fakeConsumerGroup) ResumeAll()                           {}

func TestNew_MissingConfig(t *testing.T) {
	origJWT := os.Getenv("JWT_SECRET")
	defer func() { _ = os.Setenv("JWT_SECRET", origJWT) }()
	require.NoError(t, os.Unsetenv("JWT_SECRET"))

	application, err := app.New(fx.NopLogger)
	require.Error(t, err)
	require.NotNil(t, application)
	assert.ErrorIs(t, err, config.ErrMissingJWTSecret)
}

func TestNew_ShortJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "short")

	application, err := app.New(fx.NopLogger)
	require.Error(t, err)
	require.NotNil(t, application)
	assert.ErrorIs(t, err, config.ErrJWTSecretTooShort)
}

func TestNew_ClickHouseUnreachable(t *testing.T) {
	t.Setenv("JWT_SECRET", "valid-secret-key-32-bytes-long!!")
	t.Setenv("CLICKHOUSE_DSN", "127.0.0.1:1")

	application, err := app.New(fx.NopLogger)
	require.Error(t, err)
	require.NotNil(t, application)
	assert.Contains(t, err.Error(), "clickhouse")
}

func TestNew_DIWiring(t *testing.T) {
	t.Setenv("JWT_SECRET", "valid-secret-key-32-bytes-long!!")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockAnalyticsUsecase(ctrl)
	mockUC.EXPECT().Flush(gomock.Any()).Return(nil).AnyTimes()

	fakeGroup := &fakeConsumerGroup{}
	fakeConsumer := consumer.NewConsumerFromGroup(fakeGroup, []string{"order-events"}, mockUC, zap.NewNop())

	application, err := app.New(
		fx.NopLogger,
		fx.Decorate(func() usecase.AnalyticsUsecase { return mockUC }),
		fx.Replace(fakeConsumer),
		fx.Replace(zap.NewNop()),
		fx.Replace(&config.Config{GRPCPort: 0, MetricsPort: 0}),
	)
	require.NoError(t, err)
	require.NotNil(t, application)

	startCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, application.Start(startCtx))

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, application.Stop(stopCtx))

	assert.True(t, fakeGroup.closed)
}
