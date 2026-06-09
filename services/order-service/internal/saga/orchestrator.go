package saga

import (
	"context"
	"fmt"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Orchestrator struct {
	orderRepo     repository.OrderRepository
	inventoryAddr string
	paymentAddr   string
	log           *zap.Logger
}

func NewOrchestrator(orderRepo repository.OrderRepository, inventoryAddr, paymentAddr string, log *zap.Logger) *Orchestrator {
	return &Orchestrator{
		orderRepo:     orderRepo,
		inventoryAddr: inventoryAddr,
		paymentAddr:   paymentAddr,
		log:           log,
	}
}

func (o *Orchestrator) ProcessOrder(ctx context.Context, order *domain.Order) error {
	invConn, err := grpc.NewClient(o.inventoryAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("connect inventory: %w", err)
	}
	defer invConn.Close()
	invClient := inventoryv1.NewInventoryServiceClient(invConn)

	if err := o.orderRepo.UpdateStatus(ctx, order.ID, "awaiting_payment"); err != nil {
		return fmt.Errorf("update status awaiting_payment: %w", err)
	}

	for _, item := range order.Items {
		_, err := invClient.Reserve(ctx, &inventoryv1.ReserveRequest{
			ProductId: item.ProductID.String(),
			Quantity:  int32(item.Quantity),
			OrderId:   order.ID.String(),
		})
		if err != nil {
			o.compensateInventory(ctx, invClient, order)
			_ = o.orderRepo.UpdateStatus(ctx, order.ID, "cancelled")
			return fmt.Errorf("reserve inventory product %s: %w", item.ProductID, err)
		}
	}

	payConn, err := grpc.NewClient(o.paymentAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		o.compensateInventory(ctx, invClient, order)
		_ = o.orderRepo.UpdateStatus(ctx, order.ID, "cancelled")
		return fmt.Errorf("connect payment: %w", err)
	}
	defer payConn.Close()
	payClient := paymentv1.NewPaymentServiceClient(payConn)

	_, err = payClient.ProcessPayment(ctx, &paymentv1.ProcessPaymentRequest{
		OrderId: order.ID.String(),
		UserId:  order.UserID.String(),
		Amount:  order.TotalAmount,
	})
	if err != nil {
		o.compensateInventory(ctx, invClient, order)
		_ = o.orderRepo.UpdateStatus(ctx, order.ID, "cancelled")
		return fmt.Errorf("process payment: %w", err)
	}

	if err := o.orderRepo.UpdateStatus(ctx, order.ID, "confirmed"); err != nil {
		return fmt.Errorf("update status confirmed: %w", err)
	}

	return nil
}

func (o *Orchestrator) compensateInventory(ctx context.Context, invClient inventoryv1.InventoryServiceClient, order *domain.Order) {
	for _, item := range order.Items {
		_, err := invClient.Release(ctx, &inventoryv1.ReleaseRequest{
			ProductId: item.ProductID.String(),
			Quantity:  int32(item.Quantity),
			OrderId:   order.ID.String(),
		})
		if err != nil {
			o.log.Error("failed to release inventory", zap.Error(err), zap.String("product_id", item.ProductID.String()))
		}
	}
}
