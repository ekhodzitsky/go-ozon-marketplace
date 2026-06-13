//go:build integration

package tests

import (
	"context"
	"fmt"
	"testing"

	userv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/user/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestUserServicePostgres(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()

	dsn := StartPostgres(ctx, t)

	RunMigrations(ctx, t, dsn, "../services/user-service/migrations")

	jwtSecret := "this-is-a-very-long-test-secret-for-integration-tests-only"

	grpcPort := GetFreePort(t)
	metricsPort := GetFreePort(t)
	StartService(t, "../services/user-service", []string{
		"POSTGRES_DSN=" + dsn,
		fmt.Sprintf("GRPC_PORT=%d", grpcPort),
		fmt.Sprintf("METRICS_PORT=%d", metricsPort),
		"JWT_SECRET=" + jwtSecret,
	})

	addr := fmt.Sprintf("127.0.0.1:%d", grpcPort)
	WaitForGRPC(t, addr)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect to user-service: %v", err)
	}
	defer func() { _ = conn.Close() }()

	client := userv1.NewUserServiceClient(conn)

	// Test Register
	regResp, err := client.Register(ctx, NewUserRequestBuilder().
		WithEmail("test@example.com").
		WithPassword("password123").
		WithName("Test User").
		BuildRegister())
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if regResp.UserId == "" {
		t.Fatal("expected user id after register")
	}

	// Test Login
	loginResp, err := client.Login(ctx, NewUserRequestBuilder().
		WithEmail("test@example.com").
		WithPassword("password123").
		BuildLogin())
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if loginResp.Token == "" {
		t.Fatal("expected token after login")
	}

	// Test GetUser: the caller is identified from the auth context.
	authCtx := AuthContext(ctx, regResp.UserId, jwtSecret)
	getResp, err := client.GetUser(authCtx, NewGetUserRequestBuilder().Build())
	if err != nil {
		t.Fatalf("get user failed: %v", err)
	}
	if getResp.Email != "test@example.com" {
		t.Fatalf("expected email test@example.com, got %s", getResp.Email)
	}
	if getResp.Name != "Test User" {
		t.Fatalf("expected name Test User, got %s", getResp.Name)
	}
}
