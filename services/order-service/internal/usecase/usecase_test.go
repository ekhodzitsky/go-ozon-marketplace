package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/unitofwork"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/usecase"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func testLogger(t *testing.T) *zap.Logger {
	t.Helper()
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	return logger
}

func newTestOrderUsecase(t *testing.T, ctrl *gomock.Controller) (
	usecase.OrderUsecase,
	*mocks.MockUnitOfWork,
	*mocks.MockOrderRepository,
	*mocks.MockOutboxRepository,
	*mocks.MockSagaRepository,
	*mocks.MockInventoryClient,
	*mocks.MockPaymentClient,
	*mocks.MockCatalogClient,
) {
	uow := mocks.NewMockUnitOfWork(ctrl)
	orderRepo := mocks.NewMockOrderRepository(ctrl)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	sagaRepo := mocks.NewMockSagaRepository(ctrl)
	invClient := mocks.NewMockInventoryClient(ctrl)
	payClient := mocks.NewMockPaymentClient(ctrl)
	catalogClient := mocks.NewMockCatalogClient(ctrl)

	orchestrator := saga.NewOrchestrator(orderRepo, sagaRepo, invClient, payClient, testLogger(t), 100*time.Millisecond, 100*time.Millisecond)

	uowFactory := func() unitofwork.UnitOfWork { return uow }

	uc := usecase.NewOrderUsecase(
		uowFactory,
		orderRepo,
		outboxRepo,
		sagaRepo,
		orchestrator,
		invClient,
		payClient,
		catalogClient,
		nil,
		100*time.Millisecond,
		100*time.Millisecond,
	)
	return uc, uow, orderRepo, outboxRepo, sagaRepo, invClient, payClient, catalogClient
}

func validItem() domain.OrderItem {
	return domain.OrderItem{
		ProductID: uuid.New(),
		Quantity:  2,
		Price:     500,
	}
}

func TestOrderUsecase_CreateOrder_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, uow, orderRepo, outboxRepo, sagaRepo, invClient, payClient, catalogClient := newTestOrderUsecase(t, ctrl)

	item := validItem()
	items := []domain.OrderItem{item}

	catalogClient.EXPECT().GetProduct(gomock.Any(), item.ProductID.String()).Return(&catalogv1.Product{
		ProductId:  item.ProductID.String(),
		PriceCents: item.Price,
	}, nil).Times(1)

	uow.EXPECT().Begin(gomock.Any()).Return(nil).Times(1)
	uow.EXPECT().OrderRepo().Return(orderRepo).AnyTimes()
	uow.EXPECT().OutboxRepo().Return(outboxRepo).AnyTimes()
	uow.EXPECT().Rollback(gomock.Any()).Return(nil).AnyTimes()
	uow.EXPECT().Commit(gomock.Any()).Return(nil).Times(1)

	orderRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	outboxRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	sagaRepo.EXPECT().GetByOrderID(gomock.Any(), gomock.Any()).Return(nil, apperrors.ErrNotFound)
	sagaRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	orderRepo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any(), domain.OrderStatusAwaitingPayment).Return(nil)
	invClient.EXPECT().Reserve(gomock.Any(), item.ProductID.String(), int32(item.Quantity), gomock.Any(), gomock.Any()).Return(nil)
	sagaRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	payClient.EXPECT().ProcessPayment(gomock.Any(), gomock.Any(), item.Price*int64(item.Quantity), gomock.Any()).Return("pay-123", nil)
	orderRepo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any(), domain.OrderStatusPaid).Return(nil)

	orderID, err := uc.CreateOrder(context.Background(), uuid.New(), items, "idemp-key")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, orderID)
}

func TestOrderUsecase_CreateOrder_MissingIdempotencyKey(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _, _, _, _ := newTestOrderUsecase(t, ctrl)

	_, err := uc.CreateOrder(context.Background(), uuid.New(), []domain.OrderItem{validItem()}, "")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrInvalidArgument)
}

func TestOrderUsecase_CreateOrder_EmptyItems(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _, _, _, _ := newTestOrderUsecase(t, ctrl)

	_, err := uc.CreateOrder(context.Background(), uuid.New(), nil, "idemp-key")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrInvalidArgument)
}

func TestOrderUsecase_CreateOrder_InvalidQuantity(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _, _, _, _ := newTestOrderUsecase(t, ctrl)

	item := validItem()
	item.Quantity = 0

	_, err := uc.CreateOrder(context.Background(), uuid.New(), []domain.OrderItem{item}, "idemp-key")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrInvalidArgument)
}

func TestOrderUsecase_CreateOrder_InvalidPrice(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _, _, _, _ := newTestOrderUsecase(t, ctrl)

	item := validItem()
	item.Price = 0

	_, err := uc.CreateOrder(context.Background(), uuid.New(), []domain.OrderItem{item}, "idemp-key")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrInvalidArgument)
}

func TestOrderUsecase_CreateOrder_TamperedLowerPrice(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _, _, _, catalogClient := newTestOrderUsecase(t, ctrl)

	productID := uuid.New()
	items := []domain.OrderItem{{ProductID: productID, Quantity: 1, Price: 1}}

	catalogClient.EXPECT().GetProduct(gomock.Any(), productID.String()).Return(&catalogv1.Product{
		ProductId:  productID.String(),
		PriceCents: 10000,
	}, nil).Times(1)

	orderID, err := uc.CreateOrder(context.Background(), uuid.New(), items, "idemp-key")
	require.Error(t, err)
	assert.Equal(t, uuid.Nil, orderID)
	assert.ErrorIs(t, err, apperrors.ErrInvalidArgument)
}

func TestOrderUsecase_CreateOrder_TamperedHigherPrice(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _, _, _, catalogClient := newTestOrderUsecase(t, ctrl)

	productID := uuid.New()
	items := []domain.OrderItem{{ProductID: productID, Quantity: 1, Price: 20000}}

	catalogClient.EXPECT().GetProduct(gomock.Any(), productID.String()).Return(&catalogv1.Product{
		ProductId:  productID.String(),
		PriceCents: 10000,
	}, nil).Times(1)

	orderID, err := uc.CreateOrder(context.Background(), uuid.New(), items, "idemp-key")
	require.Error(t, err)
	assert.Equal(t, uuid.Nil, orderID)
	assert.ErrorIs(t, err, apperrors.ErrInvalidArgument)
}

func TestOrderUsecase_CreateOrder_ProductNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _, _, _, catalogClient := newTestOrderUsecase(t, ctrl)

	productID := uuid.New()
	items := []domain.OrderItem{{ProductID: productID, Quantity: 1, Price: 1000}}

	catalogClient.EXPECT().GetProduct(gomock.Any(), productID.String()).Return(nil, errors.New("product not found")).Times(1)

	orderID, err := uc.CreateOrder(context.Background(), uuid.New(), items, "idemp-key")
	require.Error(t, err)
	assert.Equal(t, uuid.Nil, orderID)
}

func TestOrderUsecase_CreateOrder_UowBeginError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, uow, _, _, _, _, _, catalogClient := newTestOrderUsecase(t, ctrl)

	item := validItem()
	catalogClient.EXPECT().GetProduct(gomock.Any(), item.ProductID.String()).Return(&catalogv1.Product{
		ProductId:  item.ProductID.String(),
		PriceCents: item.Price,
	}, nil).Times(1)

	uow.EXPECT().Begin(gomock.Any()).Return(errors.New("begin failed"))
	uow.EXPECT().Rollback(gomock.Any()).Return(nil).AnyTimes()

	_, err := uc.CreateOrder(context.Background(), uuid.New(), []domain.OrderItem{item}, "idemp-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "begin uow")
}

func TestOrderUsecase_CreateOrder_CreateOrderError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, uow, orderRepo, _, _, _, _, catalogClient := newTestOrderUsecase(t, ctrl)

	item := validItem()
	catalogClient.EXPECT().GetProduct(gomock.Any(), item.ProductID.String()).Return(&catalogv1.Product{
		ProductId:  item.ProductID.String(),
		PriceCents: item.Price,
	}, nil).Times(1)

	uow.EXPECT().Begin(gomock.Any()).Return(nil)
	uow.EXPECT().OrderRepo().Return(orderRepo)
	orderRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("insert failed"))
	uow.EXPECT().Rollback(gomock.Any()).Return(nil)

	_, err := uc.CreateOrder(context.Background(), uuid.New(), []domain.OrderItem{item}, "idemp-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create order")
}

func TestOrderUsecase_CreateOrder_CreateOutboxError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, uow, orderRepo, outboxRepo, _, _, _, catalogClient := newTestOrderUsecase(t, ctrl)

	item := validItem()
	catalogClient.EXPECT().GetProduct(gomock.Any(), item.ProductID.String()).Return(&catalogv1.Product{
		ProductId:  item.ProductID.String(),
		PriceCents: item.Price,
	}, nil).Times(1)

	uow.EXPECT().Begin(gomock.Any()).Return(nil)
	uow.EXPECT().OrderRepo().Return(orderRepo)
	orderRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	uow.EXPECT().OutboxRepo().Return(outboxRepo)
	outboxRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("outbox failed"))
	uow.EXPECT().Rollback(gomock.Any()).Return(nil)

	_, err := uc.CreateOrder(context.Background(), uuid.New(), []domain.OrderItem{item}, "idemp-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create outbox event")
}

func TestOrderUsecase_CreateOrder_CommitError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, uow, orderRepo, outboxRepo, _, _, _, catalogClient := newTestOrderUsecase(t, ctrl)

	item := validItem()
	catalogClient.EXPECT().GetProduct(gomock.Any(), item.ProductID.String()).Return(&catalogv1.Product{
		ProductId:  item.ProductID.String(),
		PriceCents: item.Price,
	}, nil).Times(1)

	uow.EXPECT().Begin(gomock.Any()).Return(nil)
	uow.EXPECT().OrderRepo().Return(orderRepo)
	orderRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	uow.EXPECT().OutboxRepo().Return(outboxRepo)
	outboxRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	uow.EXPECT().Commit(gomock.Any()).Return(errors.New("commit failed"))
	uow.EXPECT().Rollback(gomock.Any()).Return(nil)

	_, err := uc.CreateOrder(context.Background(), uuid.New(), []domain.OrderItem{item}, "idemp-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "commit uow")
}

func TestOrderUsecase_CreateOrder_SagaReserveError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, uow, orderRepo, outboxRepo, sagaRepo, invClient, _, catalogClient := newTestOrderUsecase(t, ctrl)

	item := domain.OrderItem{ProductID: uuid.New(), Quantity: 1, Price: 100}

	catalogClient.EXPECT().GetProduct(gomock.Any(), item.ProductID.String()).Return(&catalogv1.Product{
		ProductId:  item.ProductID.String(),
		PriceCents: item.Price,
	}, nil).Times(1)

	uow.EXPECT().Begin(gomock.Any()).Return(nil)
	uow.EXPECT().OrderRepo().Return(orderRepo)
	orderRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	uow.EXPECT().OutboxRepo().Return(outboxRepo)
	outboxRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	uow.EXPECT().Commit(gomock.Any()).Return(nil)
	uow.EXPECT().Rollback(gomock.Any()).Return(nil)

	sagaRepo.EXPECT().GetByOrderID(gomock.Any(), gomock.Any()).Return(nil, apperrors.ErrNotFound)
	sagaRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	orderRepo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any(), domain.OrderStatusAwaitingPayment).Return(nil)
	sagaRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	invClient.EXPECT().Reserve(gomock.Any(), item.ProductID.String(), int32(item.Quantity), gomock.Any(), gomock.Any()).Return(errors.New("reserve failed"))
	orderRepo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any(), domain.OrderStatusCancelled).Return(nil)

	id, err := uc.CreateOrder(context.Background(), uuid.New(), []domain.OrderItem{item}, "idemp-key")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, id)
}

func TestOrderUsecase_GetOrder_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, orderRepo, _, _, _, _, _ := newTestOrderUsecase(t, ctrl)

	id := uuid.New()
	orderRepo.EXPECT().GetByID(gomock.Any(), id).Return(&domain.Order{ID: id}, nil)

	order, err := uc.GetOrder(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, id, order.ID)
}

func TestOrderUsecase_GetOrder_Error(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, orderRepo, _, _, _, _, _ := newTestOrderUsecase(t, ctrl)

	id := uuid.New()
	orderRepo.EXPECT().GetByID(gomock.Any(), id).Return(nil, errors.New("db error"))

	_, err := uc.GetOrder(context.Background(), id)
	require.Error(t, err)
	assert.Equal(t, "db error", err.Error())
}

func TestOrderUsecase_ListOrders_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, orderRepo, _, _, _, _, _ := newTestOrderUsecase(t, ctrl)

	userID := uuid.New()
	orderRepo.EXPECT().ListByUser(gomock.Any(), userID, 1, 10).Return([]domain.Order{{ID: uuid.New()}}, 1, nil)

	orders, total, err := uc.ListOrders(context.Background(), userID, 1, 10)
	require.NoError(t, err)
	assert.Len(t, orders, 1)
	assert.Equal(t, 1, total)
}

func TestOrderUsecase_ListOrders_Error(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, orderRepo, _, _, _, _, _ := newTestOrderUsecase(t, ctrl)

	userID := uuid.New()
	orderRepo.EXPECT().ListByUser(gomock.Any(), userID, 1, 10).Return(nil, 0, errors.New("db error"))

	_, _, err := uc.ListOrders(context.Background(), userID, 1, 10)
	require.Error(t, err)
	assert.Equal(t, "db error", err.Error())
}

func TestOrderUsecase_CancelOrder_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, orderRepo, _, _, _, _, _ := newTestOrderUsecase(t, ctrl)

	id := uuid.New()
	orderRepo.EXPECT().GetByID(gomock.Any(), id).Return(nil, apperrors.ErrNotFound)

	err := uc.CancelOrder(context.Background(), id)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
}

func TestOrderUsecase_CancelOrder_AlreadyCancelled(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, orderRepo, _, _, _, _, _ := newTestOrderUsecase(t, ctrl)

	id := uuid.New()
	orderRepo.EXPECT().GetByID(gomock.Any(), id).Return(&domain.Order{ID: id, Status: domain.OrderStatusCancelled}, nil)

	err := uc.CancelOrder(context.Background(), id)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrInvalidArgument)
}

func TestOrderUsecase_CancelOrder_DirectlyCancellable(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, orderRepo, _, _, invClient, _, _ := newTestOrderUsecase(t, ctrl)

	id := uuid.New()
	productID := uuid.New()
	order := &domain.Order{
		ID:     id,
		Status: domain.OrderStatusPending,
		Items:  []domain.OrderItem{{ProductID: productID, Quantity: 2}},
	}

	orderRepo.EXPECT().GetByID(gomock.Any(), id).Return(order, nil)
	orderRepo.EXPECT().UpdateStatus(gomock.Any(), id, domain.OrderStatusCancelled).Return(nil)
	invClient.EXPECT().Release(gomock.Any(), productID.String(), int32(2), id.String(), gomock.Any()).Return(nil)

	err := uc.CancelOrder(context.Background(), id)
	require.NoError(t, err)
}

func TestOrderUsecase_CancelOrder_PaidWithRefund(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, orderRepo, _, sagaRepo, invClient, payClient, _ := newTestOrderUsecase(t, ctrl)

	id := uuid.New()
	productID := uuid.New()
	order := &domain.Order{
		ID:     id,
		Status: domain.OrderStatusPaid,
		Items:  []domain.OrderItem{{ProductID: productID, Quantity: 1}},
	}
	s := &domain.Saga{PaymentID: "pay-123"}

	orderRepo.EXPECT().GetByID(gomock.Any(), id).Return(order, nil)
	sagaRepo.EXPECT().GetByOrderID(gomock.Any(), id).Return(s, nil)
	payClient.EXPECT().Refund(gomock.Any(), "pay-123", gomock.Any()).Return(nil)
	invClient.EXPECT().Release(gomock.Any(), productID.String(), int32(1), id.String(), gomock.Any()).Return(nil)
	orderRepo.EXPECT().UpdateStatus(gomock.Any(), id, domain.OrderStatusCancelled).Return(nil)

	err := uc.CancelOrder(context.Background(), id)
	require.NoError(t, err)
}

func TestOrderUsecase_CancelOrder_PaidRefundFails(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, orderRepo, _, sagaRepo, _, payClient, _ := newTestOrderUsecase(t, ctrl)

	id := uuid.New()
	productID := uuid.New()
	order := &domain.Order{
		ID:     id,
		Status: domain.OrderStatusPaid,
		Items:  []domain.OrderItem{{ProductID: productID, Quantity: 1}},
	}
	s := &domain.Saga{PaymentID: "pay-123"}

	orderRepo.EXPECT().GetByID(gomock.Any(), id).Return(order, nil)
	sagaRepo.EXPECT().GetByOrderID(gomock.Any(), id).Return(s, nil)
	payClient.EXPECT().Refund(gomock.Any(), "pay-123", gomock.Any()).Return(errors.New("refund declined"))

	err := uc.CancelOrder(context.Background(), id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refund payment")
}

func TestOrderUsecase_CancelOrder_SagaNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, orderRepo, _, sagaRepo, _, _, _ := newTestOrderUsecase(t, ctrl)

	id := uuid.New()
	order := &domain.Order{
		ID:     id,
		Status: domain.OrderStatusPaid,
		Items:  []domain.OrderItem{{ProductID: uuid.New(), Quantity: 1}},
	}

	orderRepo.EXPECT().GetByID(gomock.Any(), id).Return(order, nil)
	sagaRepo.EXPECT().GetByOrderID(gomock.Any(), id).Return(nil, apperrors.ErrNotFound)

	err := uc.CancelOrder(context.Background(), id)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrFailedPrecondition)
}

func TestOrderUsecase_CancelOrder_NonCancellableStatus(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, orderRepo, _, _, _, _, _ := newTestOrderUsecase(t, ctrl)

	id := uuid.New()
	order := &domain.Order{
		ID:     id,
		Status: domain.OrderStatusRefunded,
	}

	orderRepo.EXPECT().GetByID(gomock.Any(), id).Return(order, nil)

	err := uc.CancelOrder(context.Background(), id)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrFailedPrecondition)
}

func TestOrderUsecase_UpdateOrderStatus_InvalidTransition(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, orderRepo, _, _, _, _, _ := newTestOrderUsecase(t, ctrl)

	id := uuid.New()
	orderRepo.EXPECT().GetByID(gomock.Any(), id).Return(&domain.Order{ID: id, Status: domain.OrderStatusCancelled}, nil)

	err := uc.UpdateOrderStatus(context.Background(), id, domain.OrderStatusPaid)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrInvalidArgument)
}

func TestOrderUsecase_UpdateOrderStatus_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, orderRepo, _, _, _, _, _ := newTestOrderUsecase(t, ctrl)

	id := uuid.New()
	orderRepo.EXPECT().GetByID(gomock.Any(), id).Return(&domain.Order{ID: id, Status: domain.OrderStatusPending}, nil)
	orderRepo.EXPECT().UpdateStatus(gomock.Any(), id, domain.OrderStatusAwaitingPayment).Return(nil)

	err := uc.UpdateOrderStatus(context.Background(), id, domain.OrderStatusAwaitingPayment)
	require.NoError(t, err)
}

func TestOrderUsecase_UpdateOrderStatus_UpdateError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, orderRepo, _, _, _, _, _ := newTestOrderUsecase(t, ctrl)

	id := uuid.New()
	orderRepo.EXPECT().GetByID(gomock.Any(), id).Return(&domain.Order{ID: id, Status: domain.OrderStatusPending}, nil)
	orderRepo.EXPECT().UpdateStatus(gomock.Any(), id, domain.OrderStatusAwaitingPayment).Return(errors.New("db error"))

	err := uc.UpdateOrderStatus(context.Background(), id, domain.OrderStatusAwaitingPayment)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update order status")
}
