//go:build chaos

package chaos

import (
	"context"
	"testing"
	"time"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/tests"
	"github.com/google/uuid"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func newTestCircuitBreaker() *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "chaos-test",
		MaxRequests: 2,
		Interval:    0,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
	})
}

func TestCircuitBreakerOpens(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}
	dockerComposeUp(t)
	runMigrations(t)

	ctx := context.Background()
	addr := "localhost:50052"
	tests.WaitForGRPC(t, addr)

	cb := newTestCircuitBreaker()
	interceptor := func(cb *gobreaker.CircuitBreaker) grpc.UnaryClientInterceptor {
		return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			_, err := cb.Execute(func() (interface{}, error) {
				return nil, invoker(ctx, method, req, reply, cc, opts...)
			})
			return err
		}
	}

	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(interceptor(cb)),
	)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = conn.Close() }()
	client := inventoryv1.NewInventoryServiceClient(conn)

	productID := uuid.NewString()

	// Kill inventory-service
	dockerKill(t, containerName("inventory-service"))

	openSeen := false
	for i := 0; i < 10; i++ {
		_, err := client.GetStock(ctx, tests.NewGetStockRequestBuilder().WithProductID(productID).Build())
		if err != nil && cb.State() == gobreaker.StateOpen {
			openSeen = true
		}
	}

	if !openSeen {
		t.Fatalf("expected circuit breaker to open after failures, state=%v", cb.State())
	}

	// Restart inventory-service and wait for it to be running
	dockerStart(t, containerName("inventory-service"))
	waitForContainer(t, containerName("inventory-service"), 30*time.Second)
	tests.WaitForGRPC(t, addr)

	// Wait for CB timeout to transition to half-open
	t.Log("waiting 30s for CB timeout...")
	time.Sleep(31 * time.Second)

	// First request after timeout should transition to half-open
	_, err = client.GetStock(ctx, tests.NewGetStockRequestBuilder().WithProductID(productID).Build())
	if err != nil {
		t.Fatalf("expected half-open request to succeed after service restart: %v", err)
	}

	if cb.State() != gobreaker.StateHalfOpen && cb.State() != gobreaker.StateClosed {
		t.Fatalf("expected state half-open or closed after successful request, got %v", cb.State())
	}

	// Send one more successful request to close the circuit
	_, err = client.GetStock(ctx, tests.NewGetStockRequestBuilder().WithProductID(productID).Build())
	if err != nil {
		t.Fatalf("expected second request to succeed: %v", err)
	}

	if cb.State() != gobreaker.StateClosed {
		t.Fatalf("expected circuit breaker to be closed after 2 successes, got %v", cb.State())
	}
}
