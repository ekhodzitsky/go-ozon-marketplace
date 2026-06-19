package app_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/fxmodules"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/kafka"
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

// disabledGRPCServerConfig disables the gRPC server in DI wiring tests.
type disabledGRPCServerConfig struct{}

func (disabledGRPCServerConfig) GetGRPCPort() int     { return 0 }
func (disabledGRPCServerConfig) GetMetricsPort() int  { return 0 }
func (disabledGRPCServerConfig) GetCertPath() string  { return "" }
func (disabledGRPCServerConfig) GetJWTSecret() string { return "" }

func TestLoadConfig_MissingJWTSecret(t *testing.T) {
	origJWT := os.Getenv("JWT_SECRET")
	defer func() { _ = os.Setenv("JWT_SECRET", origJWT) }()
	require.NoError(t, os.Unsetenv("JWT_SECRET"))

	_, err := config.Load()
	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrMissingJWTSecret)
}

func TestLoadConfig_ShortJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "short")

	_, err := config.Load()
	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrJWTSecretTooShort)
}

func TestLoadConfig_InvalidClickHouseDSN(t *testing.T) {
	t.Setenv("JWT_SECRET", "valid-secret-key-32-bytes-long!!")
	t.Setenv("CLICKHOUSE_DSN", "not-a-valid-dsn")

	_, err := config.Load()
	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrInvalidClickHouseDSN)
}

func TestNew_DIWiring(t *testing.T) {
	t.Setenv("JWT_SECRET", "valid-secret-key-32-bytes-long!!")

	cfg, err := config.Load()
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockAnalyticsUsecase(ctrl)
	mockUC.EXPECT().Flush(gomock.Any()).Return(nil).AnyTimes()

	fakeGroup := &fakeConsumerGroup{}
	processor := consumer.NewProcessor(mockUC, zap.NewNop())
	fakeConsumer := kafka.NewConsumerFromGroup(fakeGroup, kafka.Config{Topics: []string{"order-events"}}, processor, zap.NewNop())

	application := app.New(cfg,
		fx.NopLogger,
		fx.Decorate(func() usecase.AnalyticsUsecase { return mockUC }),
		fx.Replace(fakeConsumer),
		fx.Replace(zap.NewNop()),
		fx.Replace(fxmodules.GRPCServerConfig(disabledGRPCServerConfig{})),
	)
	require.NotNil(t, application)

	startCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, application.Start(startCtx))

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, application.Stop(stopCtx))

	assert.True(t, fakeGroup.closed)
}
