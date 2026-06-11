//go:build chaos

package chaos

import (
	"context"
	"testing"
	"time"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/circuitbreaker"
	"github.com/ekhodzitsky/go-ozon-marketplace/tests"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestCircuitBreakerOpens(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}
	dockerComposeUp(t)
	runMigrations(t)

	ctx := context.Background()
	addr := "localhost:50052"
	tests.WaitForGRPC(t, addr)

	cb := circuitbreaker.New(5, 2, 30*time.Second)
	interceptor := func(cb *circuitbreaker.CircuitBreaker) grpc.UnaryClientInterceptor {
		return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			return cb.Call(func() error {
				return invoker(ctx, method, req, reply, cc, opts...)
			})
		}
	}

	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(interceptor(cb)),
	)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()
	client := inventoryv1.NewInventoryServiceClient(conn)

	productID := uuid.New().String()

	// Kill inventory-service
	dockerKill(t, "go-ozon-marketplace-inventory-service-1")

	openSeen := false
	for i := 0; i < 10; i++ {
		_, err := client.GetStock(ctx, &inventoryv1.GetStockRequest{ProductId: productID})
		if err != nil {
			if cb.State() == circuitbreaker.StateOpen {
				openSeen = true
			}
		}
	}

	if !openSeen {
		t.Fatalf("expected circuit breaker to open after failures, state=%v", cb.State())
	}

	// Restart inventory-service
	dockerStart(t, "go-ozon-marketplace-inventory-service-1")
	tests.WaitForGRPC(t, addr)

	// Wait for CB timeout to transition to half-open
	t.Log("waiting 30s for CB timeout...")
	time.Sleep(31 * time.Second)

	// First request after timeout should transition to half-open
	_, err = client.GetStock(ctx, &inventoryv1.GetStockRequest{ProductId: productID})
	if err != nil {
		t.Fatalf("expected half-open request to succeed after service restart: %v", err)
	}

	if cb.State() != circuitbreaker.StateHalfOpen && cb.State() != circuitbreaker.StateClosed {
		t.Fatalf("expected state half-open or closed after successful request, got %v", cb.State())
	}

	// Send one more successful request to close the circuit
	_, err = client.GetStock(ctx, &inventoryv1.GetStockRequest{ProductId: productID})
	if err != nil {
		t.Fatalf("expected second request to succeed: %v", err)
	}

	if cb.State() != circuitbreaker.StateClosed {
		t.Fatalf("expected circuit breaker to be closed after 2 successes, got %v", cb.State())
	}
}
