package steps_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/steps"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestReserveInventoryStep_Name(t *testing.T) {
	t.Parallel()

	step := steps.NewReserveInventoryStep(nil)
	assert.Equal(t, "inventory", step.Name())
}

func TestReserveInventoryStep_Execute_ReservesNextItem(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockInventoryClient(ctrl)
	step := steps.NewReserveInventoryStep(client)
	orderID := uuid.New()
	productID := uuid.New()
	order := &domain.Order{
		ID: orderID,
		Items: []domain.OrderItem{
			{ProductID: productID, Quantity: 5},
		},
	}
	saga := &domain.Saga{Status: domain.SagaStatusReserving}

	client.EXPECT().Reserve(gomock.Any(), productID.String(), int32(5), orderID.String(), gomock.Any()).Return(nil)

	err := step.Execute(context.Background(), saga, order, "key")
	require.NoError(t, err)
	require.Len(t, saga.ReservedItems, 1)
	assert.Equal(t, productID.String(), saga.ReservedItems[0].ProductID)
	assert.Equal(t, int32(5), saga.ReservedItems[0].Quantity)
	assert.Equal(t, domain.SagaStatusReserving, saga.Status)
}

func TestReserveInventoryStep_Execute_FinalizesWhenAllReserved(t *testing.T) {
	t.Parallel()

	step := steps.NewReserveInventoryStep(nil)
	order := &domain.Order{
		ID: uuid.New(),
		Items: []domain.OrderItem{
			{ProductID: uuid.New(), Quantity: 1},
		},
	}
	saga := &domain.Saga{
		Status: domain.SagaStatusReserving,
		ReservedItems: []domain.SagaReservedItem{
			{ProductID: order.Items[0].ProductID.String(), Quantity: 1},
		},
	}

	err := step.Execute(context.Background(), saga, order, "key")
	require.NoError(t, err)
	assert.Equal(t, domain.SagaStatusReserved, saga.Status)
	assert.Equal(t, "reserved", saga.CurrentStep)
}

func TestReserveInventoryStep_Execute_PropagatesReserveError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockInventoryClient(ctrl)
	step := steps.NewReserveInventoryStep(client)
	order := &domain.Order{
		ID: uuid.New(),
		Items: []domain.OrderItem{
			{ProductID: uuid.New(), Quantity: 2},
		},
	}
	saga := &domain.Saga{Status: domain.SagaStatusReserving}
	want := errors.New("out of stock")

	client.EXPECT().Reserve(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(want)

	err := step.Execute(context.Background(), saga, order, "key")
	assert.ErrorIs(t, err, want)
}

func TestReserveInventoryStep_Compensate_ReleasesReservedItems(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockInventoryClient(ctrl)
	step := steps.NewReserveInventoryStep(client)
	orderID := uuid.New()
	order := &domain.Order{ID: orderID}
	saga := &domain.Saga{
		ReservedItems: []domain.SagaReservedItem{
			{ProductID: "p1", Quantity: 2},
			{ProductID: "p2", Quantity: 3},
		},
	}

	client.EXPECT().Release(gomock.Any(), "p1", int32(2), orderID.String(), gomock.Any()).Return(nil)
	client.EXPECT().Release(gomock.Any(), "p2", int32(3), orderID.String(), gomock.Any()).Return(nil)

	err := step.Compensate(context.Background(), saga, order, "key")
	require.NoError(t, err)
}

func TestReserveInventoryStep_Compensate_JoinsErrors(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockInventoryClient(ctrl)
	step := steps.NewReserveInventoryStep(client)
	orderID := uuid.New()
	order := &domain.Order{ID: orderID}
	saga := &domain.Saga{
		ReservedItems: []domain.SagaReservedItem{
			{ProductID: "p1", Quantity: 1},
		},
	}
	want := errors.New("release failed")

	client.EXPECT().Release(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(want)

	err := step.Compensate(context.Background(), saga, order, "key")
	assert.ErrorIs(t, err, want)
}
