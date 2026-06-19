package usecase

import (
	"context"
	"fmt"
	"time"

	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/unitofwork"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	_ OrderUsecase = (*orderUsecase)(nil)
)

type OrderOrchestrator interface {
	ProcessOrder(ctx context.Context, order *domain.Order, idempotencyKey string) error
}

// TxManager runs business logic inside a transactional UnitOfWork.
type TxManager interface {
	Run(ctx context.Context, fn func(unitofwork.UnitOfWork) error) error
}

type orderUsecase struct {
	txm           TxManager
	orderRepo     repository.OrderRepository
	outboxRepo    repository.OutboxRepository
	sagaRepo      repository.SagaRepository
	orchestrator  OrderOrchestrator
	invClient     inventoryv1.InventoryServiceClient
	payClient     paymentv1.PaymentServiceClient
	catalogClient catalogv1.CatalogServiceClient
	redis         *redis.Client
	callTimeout   time.Duration
	queryTimeout  time.Duration
}

func NewOrderUsecase(
	txm TxManager,
	orderRepo repository.OrderRepository,
	outboxRepo repository.OutboxRepository,
	sagaRepo repository.SagaRepository,
	orchestrator OrderOrchestrator,
	invClient inventoryv1.InventoryServiceClient,
	payClient paymentv1.PaymentServiceClient,
	catalogClient catalogv1.CatalogServiceClient,
	redis *redis.Client,
	callTimeout time.Duration,
	queryTimeout time.Duration,
) OrderUsecase {
	if callTimeout == 0 {
		callTimeout = 5 * time.Second
	}
	if queryTimeout == 0 {
		queryTimeout = 3 * time.Second
	}
	return &orderUsecase{
		txm:           txm,
		orderRepo:     orderRepo,
		outboxRepo:    outboxRepo,
		sagaRepo:      sagaRepo,
		orchestrator:  orchestrator,
		invClient:     invClient,
		payClient:     payClient,
		catalogClient: catalogClient,
		redis:         redis,
		callTimeout:   callTimeout,
		queryTimeout:  queryTimeout,
	}
}

func (u *orderUsecase) CreateOrder(ctx context.Context, userID uuid.UUID, items []domain.OrderItem, idempotencyKey string) (uuid.UUID, error) {
	var total int64
	for _, item := range items {
		total += item.Price * int64(item.Quantity)
	}

	for _, item := range items {
		cCtx, cancel := context.WithTimeout(ctx, u.callTimeout)
		resp, err := u.catalogClient.GetProduct(cCtx, &catalogv1.GetProductRequest{ProductId: item.ProductID.String()})
		cancel()
		if err != nil {
			return uuid.Nil, fmt.Errorf("get product %s: %w", item.ProductID, err)
		}
		product := resp.GetProduct()
		if product == nil || product.PriceCents != item.Price {
			return uuid.Nil, fmt.Errorf("%w: price mismatch for product %s", apperrors.ErrInvalidArgument, item.ProductID)
		}
	}

	order := &domain.Order{
		ID:          uuid.New(),
		UserID:      userID,
		Items:       items,
		TotalAmount: total,
		Status:      domain.OrderStatusPending,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	for i := range order.Items {
		order.Items[i].ID = uuid.New()
		order.Items[i].OrderID = order.ID
	}

	txCtx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()

	var createdID uuid.UUID
	err := u.txm.Run(txCtx, func(uow unitofwork.UnitOfWork) error {
		if err := uow.OrderRepo().Create(txCtx, order); err != nil {
			return fmt.Errorf("create order: %w", err)
		}

		event, err := outboxEventFromOrder(order)
		if err != nil {
			return err
		}

		if err := uow.OutboxRepo().Create(txCtx, event); err != nil {
			return fmt.Errorf("create outbox event: %w", err)
		}

		createdID = order.ID
		return nil
	})
	if err != nil {
		return uuid.Nil, err
	}

	u.publishOrderEvent(ctx, order.ID.String(), string(order.Status), order.UserID.String())

	sagaCtx, cancel := context.WithTimeout(ctx, u.callTimeout)
	defer cancel()
	if err := u.orchestrator.ProcessOrder(sagaCtx, order, idempotencyKey); err != nil {
		return createdID, fmt.Errorf("saga process order: %w", err)
	}

	return createdID, nil
}

func (u *orderUsecase) GetOrder(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	ctx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()
	return u.orderRepo.GetByID(ctx, id)
}

func (u *orderUsecase) ListOrders(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]domain.Order, int, error) {
	ctx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()
	return u.orderRepo.ListByUser(ctx, userID, page, pageSize)
}

func (u *orderUsecase) CancelOrder(ctx context.Context, id uuid.UUID) error {
	qCtx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()

	order, err := u.orderRepo.GetByID(qCtx, id)
	if err != nil {
		return fmt.Errorf("get order: %w", err)
	}

	if order.Status == domain.OrderStatusCancelled {
		return fmt.Errorf("%w: order already cancelled", apperrors.ErrInvalidArgument)
	}

	if order.Status.CancellableDirectly() {
		if err := u.orderRepo.UpdateStatus(qCtx, id, domain.OrderStatusCancelled); err != nil {
			return fmt.Errorf("update order status: %w", err)
		}
		u.publishOrderEvent(ctx, order.ID.String(), string(domain.OrderStatusCancelled), order.UserID.String())

		cCtx, cancel := context.WithTimeout(ctx, u.callTimeout)
		defer cancel()
		for _, item := range order.Items {
			if _, err := u.invClient.Release(cCtx, &inventoryv1.ReleaseRequest{
				ProductId:      item.ProductID.String(),
				Quantity:       int32(item.Quantity),
				OrderId:        order.ID.String(),
				IdempotencyKey: releaseIdempotencyKey(order.ID.String(), item.ProductID.String()),
			}); err != nil {
				return fmt.Errorf("release inventory for product %s: %w", item.ProductID, err)
			}
		}
		return nil
	}

	// Paid or later orders require compensation/refund.
	if order.Status == domain.OrderStatusPaid || order.Status == domain.OrderStatusProcessing || order.Status == domain.OrderStatusShipped {
		saga, err := u.sagaRepo.GetByOrderID(qCtx, order.ID)
		if err != nil {
			return fmt.Errorf("%w: saga not found for refund: %v", apperrors.ErrFailedPrecondition, err)
		}

		cCtx, cancel := context.WithTimeout(ctx, u.callTimeout)
		defer cancel()
		if saga.PaymentID != "" {
			if _, err := u.payClient.Refund(cCtx, &paymentv1.RefundRequest{
				PaymentId:      saga.PaymentID,
				IdempotencyKey: refundIdempotencyKey(order.ID.String(), saga.PaymentID),
			}); err != nil {
				return fmt.Errorf("refund payment %s: %w", saga.PaymentID, err)
			}
		}
		for _, item := range order.Items {
			if _, err := u.invClient.Release(cCtx, &inventoryv1.ReleaseRequest{
				ProductId:      item.ProductID.String(),
				Quantity:       int32(item.Quantity),
				OrderId:        order.ID.String(),
				IdempotencyKey: releaseIdempotencyKey(order.ID.String(), item.ProductID.String()),
			}); err != nil {
				return fmt.Errorf("release inventory for product %s: %w", item.ProductID, err)
			}
		}

		if err := u.orderRepo.UpdateStatus(qCtx, id, domain.OrderStatusCancelled); err != nil {
			return fmt.Errorf("update order status: %w", err)
		}
		u.publishOrderEvent(ctx, order.ID.String(), string(domain.OrderStatusCancelled), order.UserID.String())
		return nil
	}

	return fmt.Errorf("%w: order with status %s cannot be cancelled", apperrors.ErrFailedPrecondition, order.Status)
}

func (u *orderUsecase) UpdateOrderStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus) error {
	qCtx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()

	order, err := u.orderRepo.GetByID(qCtx, id)
	if err != nil {
		return fmt.Errorf("get order: %w", err)
	}

	if err := order.Status.ValidateTransition(status); err != nil {
		return fmt.Errorf("%w: %v", apperrors.ErrInvalidArgument, err)
	}

	if err := u.orderRepo.UpdateStatus(qCtx, id, status); err != nil {
		return fmt.Errorf("update order status: %w", err)
	}

	u.publishOrderEvent(ctx, order.ID.String(), string(status), order.UserID.String())
	return nil
}
