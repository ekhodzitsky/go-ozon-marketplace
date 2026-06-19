package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/graphqlclient"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
)

const (
	defaultGraphQLURL    = "http://localhost:8080/query"
	defaultOrderGRPC     = "localhost:50055"
	defaultInventoryGRPC = "localhost:50053"
	defaultPostgresDSN   = "postgres://ozon:ozonpass@localhost:5432/marketplace?sslmode=disable"
	defaultCertPath      = ""
)

type config struct {
	graphqlURL    string
	orderGRPC     string
	inventoryGRPC string
	postgresDSN   string
	certPath      string
	graphqlClient *graphqlclient.Client
}

func loadConfig() config {
	return config{
		graphqlURL:    getenv("GRAPHQL_URL", defaultGraphQLURL),
		orderGRPC:     getenv("ORDER_ADDR", defaultOrderGRPC),
		inventoryGRPC: getenv("INVENTORY_ADDR", defaultInventoryGRPC),
		postgresDSN:   getenv("POSTGRES_DSN", defaultPostgresDSN),
		certPath:      getenv("CERT_PATH", defaultCertPath),
		graphqlClient: graphqlclient.NewClient(getenv("GRAPHQL_URL", defaultGraphQLURL)),
	}
}

func getenv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func main() {
	ctx := context.Background()
	cfg := loadConfig()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 1. Categories
	categories := []string{"Electronics", "Books", "Clothing", "Home", "Sports"}

	// 2. Create 100 products via GraphQL
	fmt.Println("Creating 100 products...")
	productIDs := make([]string, 100)
	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("Product-%03d", i+1)
		desc := fmt.Sprintf("Description for product %d", i+1)
		price := float64(rng.Intn(9900)+100) / 100.0 // 10.00 - 100.00
		cat := categories[rng.Intn(len(categories))]
		id, err := createProduct(ctx, cfg, name, desc, price, []string{cat})
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create product %d: %v\n", i+1, err)
			os.Exit(1)
		}
		productIDs[i] = id
		if (i+1)%10 == 0 {
			fmt.Printf("  created %d products\n", i+1)
		}
	}

	// 3. Insert inventory stock directly into DB so that Reserve works
	fmt.Println("Inserting inventory stock...")
	pool, err := pgxpool.New(ctx, cfg.postgresDSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to postgres: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := seedInventory(ctx, pool, productIDs); err != nil {
		fmt.Fprintf(os.Stderr, "failed to seed inventory: %v\n", err)
		os.Exit(1)
	}

	// 4. Create 10 users via GraphQL (register + login)
	fmt.Println("Creating 10 users...")
	for i := 0; i < 10; i++ {
		email := fmt.Sprintf("user%d@example.com", i+1)
		name := fmt.Sprintf("User %d", i+1)
		password := generatePassword(rng)
		uid, err := register(ctx, cfg, email, password, name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to register user %d: %v\n", i+1, err)
			os.Exit(1)
		}
		if _, err := login(ctx, cfg, email, password); err != nil {
			fmt.Fprintf(os.Stderr, "failed to login user %d: %v\n", i+1, err)
			os.Exit(1)
		}
		fmt.Printf("  user %d: %s\n", i+1, uid)
	}

	// 5. Create 50 orders via gRPC
	fmt.Println("Creating 50 orders...")
	orderConn, err := newGRPCConn(ctx, cfg.orderGRPC, cfg.certPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to order service: %v\n", err)
		os.Exit(1)
	}
	defer closeConn(orderConn, "order service")
	orderClient := orderv1.NewOrderServiceClient(orderConn)

	invConn, err := newGRPCConn(ctx, cfg.inventoryGRPC, cfg.certPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to inventory service: %v\n", err)
		os.Exit(1)
	}
	defer closeConn(invConn, "inventory service")
	invClient := inventoryv1.NewInventoryServiceClient(invConn)

	for i := 0; i < 50; i++ {
		numItems := rng.Intn(3) + 1 // 1-3 items
		items := make([]*orderv1.OrderItem, numItems)
		for j := range items {
			pid := productIDs[rng.Intn(len(productIDs))]
			qty := int32(rng.Intn(5) + 1)
			priceCents := int64(rng.Intn(9900) + 100) // 10.00 - 100.00 in cents
			items[j] = &orderv1.OrderItem{
				ProductId:  pid,
				Quantity:   qty,
				PriceCents: priceCents,
			}
		}
		resp, err := orderClient.CreateOrder(ctx, &orderv1.CreateOrderRequest{
			Items:          items,
			IdempotencyKey: fmt.Sprintf("seed-create-order-%04d", i),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create order %d: %v\n", i+1, err)
			continue
		}
		if (i+1)%10 == 0 {
			fmt.Printf("  created %d orders (last %s)\n", i+1, resp.OrderId)
		}
	}

	// 6. Fill inventory via gRPC Reserve / Release
	fmt.Println("Reserving/Releasing inventory...")
	for i := 0; i < 20; i++ {
		pid := productIDs[rng.Intn(len(productIDs))]
		oid := fmt.Sprintf("seed-order-%04d", i)
		qty := int32(rng.Intn(10) + 1)
		_, err := invClient.Reserve(ctx, &inventoryv1.ReserveRequest{
			ProductId:      pid,
			Quantity:       qty,
			OrderId:        oid,
			IdempotencyKey: fmt.Sprintf("seed-reserve-%04d", i),
		})
		if err != nil {
			fmt.Printf("  reserve failed for %s: %v\n", pid, err)
			continue
		}
		fmt.Printf("  reserved %d for %s (order %s)\n", qty, pid, oid)

		// Release half of them
		if i%2 == 0 {
			_, err := invClient.Release(ctx, &inventoryv1.ReleaseRequest{
				ProductId:      pid,
				Quantity:       qty,
				OrderId:        oid,
				IdempotencyKey: fmt.Sprintf("seed-release-%04d", i),
			})
			if err != nil {
				fmt.Printf("  release failed for %s: %v\n", pid, err)
				continue
			}
			fmt.Printf("  released %d for %s (order %s)\n", qty, pid, oid)
		}
	}

	fmt.Println("Seed completed.")
}

func seedInventory(ctx context.Context, pool *pgxpool.Pool, productIDs []string) (err error) {
	batch := &pgx.Batch{}
	for _, pid := range productIDs {
		batch.Queue(`
			INSERT INTO inventory (product_id, available, reserved)
			VALUES ($1, $2, $3)
			ON CONFLICT (product_id) DO UPDATE SET available = $2, reserved = $3
		`, pid, 1000, 0)
	}

	br := pool.SendBatch(ctx, batch)
	defer func() {
		if closeErr := br.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close inventory batch: %w", closeErr)
		}
	}()

	for i := 0; i < len(productIDs); i++ {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("failed to insert inventory batch item %d: %w", i, err)
		}
	}
	return nil
}

func generatePassword(rng *rand.Rand) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	b := make([]byte, 16)
	for i := range b {
		b[i] = chars[rng.Intn(len(chars))]
	}
	return string(b)
}

func closeConn(conn *grpc.ClientConn, name string) {
	if err := conn.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to close %s connection: %v\n", name, err)
	}
}

func newGRPCConn(_ context.Context, addr, certPath string) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
	if certPath != "" {
		creds, err := server.LoadClientMTLSCredentials(
			filepath.Join(certPath, "server-cert.pem"),
			filepath.Join(certPath, "server-key.pem"),
			filepath.Join(certPath, "ca-cert.pem"),
			"",
		)
		if err != nil {
			return nil, fmt.Errorf("load tls cert from %s: %w", certPath, err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	return grpc.NewClient(addr, opts...)
}

func createProduct(ctx context.Context, cfg config, name, desc string, price float64, categories []string) (string, error) {
	cats, err := json.Marshal(categories)
	if err != nil {
		return "", err
	}
	query := fmt.Sprintf(`mutation {
		createProduct(name: %q, description: %q, price: %f, categories: %s)
	}`, name, desc, price, string(cats))

	var result struct {
		CreateProduct string `json:"createProduct"`
	}
	if err := cfg.graphqlClient.Do(ctx, query, &result); err != nil {
		return "", err
	}
	return result.CreateProduct, nil
}

func register(ctx context.Context, cfg config, email, password, name string) (string, error) {
	query := fmt.Sprintf(`mutation {
		register(email: %q, password: %q, name: %q)
	}`, email, password, name)

	var result struct {
		Register string `json:"register"`
	}
	if err := cfg.graphqlClient.Do(ctx, query, &result); err != nil {
		return "", err
	}
	return result.Register, nil
}

func login(ctx context.Context, cfg config, email, password string) (string, error) {
	query := fmt.Sprintf(`mutation {
		login(email: %q, password: %q)
	}`, email, password)

	var result struct {
		Login string `json:"login"`
	}
	if err := cfg.graphqlClient.Do(ctx, query, &result); err != nil {
		return "", err
	}
	return result.Login, nil
}
