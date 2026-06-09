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
	ctx := context.Background()

	dsn, cleanupDB := StartPostgres(ctx, t)
	defer cleanupDB()

	RunMigrations(ctx, t, dsn, "../services/user-service/migrations")

	grpcPort := GetFreePort(t)
	cmd := StartService(t, "../services/user-service", []string{
		"POSTGRES_DSN=" + dsn,
		fmt.Sprintf("GRPC_PORT=%d", grpcPort),
	})
	defer func() { _ = cmd.Process.Kill() }()

	addr := fmt.Sprintf("127.0.0.1:%d", grpcPort)
	WaitForGRPC(t, addr)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect to user-service: %v", err)
	}
	defer conn.Close()

	client := userv1.NewUserServiceClient(conn)

	// Test Register
	regResp, err := client.Register(ctx, &userv1.RegisterRequest{
		Email:    "test@example.com",
		Password: "password123",
		Name:     "Test User",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if regResp.UserId == "" {
		t.Fatal("expected user id after register")
	}

	// Test Login
	loginResp, err := client.Login(ctx, &userv1.LoginRequest{
		Email:    "test@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if loginResp.Token == "" {
		t.Fatal("expected token after login")
	}

	// Test GetUser
	getResp, err := client.GetUser(ctx, &userv1.GetUserRequest{
		UserId: regResp.UserId,
	})
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
