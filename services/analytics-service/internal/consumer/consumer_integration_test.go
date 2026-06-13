//go:build integration

package consumer_test

import (
	"context"
	"testing"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/consumer"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/mocks"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestConsumer_NewAndClose(t *testing.T) {
	ctx := context.Background()

	container, err := kafka.Run(ctx, "confluentinc/confluent-local:7.5.0")
	require.NoError(t, err)
	defer container.Terminate(ctx)

	brokers, err := container.Brokers(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, brokers)

	ctrl := mocks.NewMockAnalyticsUsecase(gomock.NewController(t))
	log := zap.NewNop()

	c, err := consumer.NewConsumer(brokers, "analytics-integration-test", []string{"order-events"}, ctrl, log)
	require.NoError(t, err)
	require.NotNil(t, c)

	require.NoError(t, c.Close())
}
