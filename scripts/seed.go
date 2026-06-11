package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
)

const (
	graphqlURL    = "http://localhost:8080/query"
	orderGRPC     = "localhost:50055"
	inventoryGRPC = "localhost:50053"
	postgresDSN   = "postgres://ozon:ozonpass@localhost:5432/marketplace?sslmode=disable"
)

func main() {
	ctx := context.Background()
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
		id := createProduct(name, desc, price, []string{cat})
		productIDs[i] = id
		if (i+1)%10 == 0 {
			fmt.Printf("  created %d products\n", i+1)
		}
	}

	// 3. Insert inventory stock directly into DB so that Reserve works
	fmt.Println("Inserting inventory stock...")
	pool, err := pgxpool.New(ctx, postgresDSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to postgres: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	for _, pid := range productIDs {
		_, err := pool.Exec(ctx, `
			INSERT INTO inventory (product_id, available, reserved)
			VALUES ($1, $2, $3)
			ON CONFLICT (product_id) DO UPDATE SET available = $2, reserved = $3
		`, pid, 1000, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to insert inventory for %s: %v\n", pid, err)
			os.Exit(1)
		}
	}

	// 4. Create 10 users via GraphQL (register + login)
	fmt.Println("Creating 10 users...")
	userIDs := make([]string, 10)
	for i := 0; i < 10; i++ {
		email := fmt.Sprintf("user%d@example.com", i+1)
		name := fmt.Sprintf("User %d", i+1)
		password := "password123"
		uid := register(email, password, name)
		_ = login(email, password)
		userIDs[i] = uid
		fmt.Printf("  user %d: %s\n", i+1, uid)
	}

	// 5. Create 50 orders via gRPC
	fmt.Println("Creating 50 orders...")
	orderConn, err := grpc.NewClient(orderGRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to order service: %v\n", err)
		os.Exit(1)
	}
	defer orderConn.Close()
	orderClient := orderv1.NewOrderServiceClient(orderConn)

	invConn, err := grpc.NewClient(inventoryGRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to inventory service: %v\n", err)
		os.Exit(1)
	}
	defer invConn.Close()
	invClient := inventoryv1.NewInventoryServiceClient(invConn)

	for i := 0; i < 50; i++ {
		uid := userIDs[rng.Intn(len(userIDs))]
		numItems := rng.Intn(3) + 1 // 1-3 items
		items := make([]*orderv1.OrderItem, numItems)
		for j := range items {
			pid := productIDs[rng.Intn(len(productIDs))]
			qty := int32(rng.Intn(5) + 1)
			price := float64(rng.Intn(9900)+100) / 100.0
			items[j] = &orderv1.OrderItem{
				ProductId: pid,
				Quantity:  qty,
				Price:     price,
			}
		}
		resp, err := orderClient.CreateOrder(ctx, &orderv1.CreateOrderRequest{
			UserId: uid,
			Items:  items,
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
			ProductId: pid,
			Quantity:  qty,
			OrderId:   oid,
		})
		if err != nil {
			fmt.Printf("  reserve failed for %s: %v\n", pid, err)
			continue
		}
		fmt.Printf("  reserved %d for %s (order %s)\n", qty, pid, oid)

		// Release half of them
		if i%2 == 0 {
			_, err := invClient.Release(ctx, &inventoryv1.ReleaseRequest{
				ProductId: pid,
				Quantity:  qty,
				OrderId:   oid,
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

func graphqlRequest(query string) map[string]interface{} {
	body, _ := json.Marshal(map[string]string{"query": query})
	resp, err := http.Post(graphqlURL, "application/json", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "graphql request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(os.Stderr, "graphql decode failed: %v\n", err)
		os.Exit(1)
	}
	if errs, ok := result["errors"].([]interface{}); ok && len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "graphql error: %v\n", errs)
		os.Exit(1)
	}
	return result
}

func createProduct(name, desc string, price float64, categories []string) string {
	cats, _ := json.Marshal(categories)
	query := fmt.Sprintf(`mutation {
		createProduct(name: %q, description: %q, price: %f, categories: %s)
	}`, name, desc, price, string(cats))
	result := graphqlRequest(query)
	data := result["data"].(map[string]interface{})
	return data["createProduct"].(string)
}

func register(email, password, name string) string {
	query := fmt.Sprintf(`mutation {
		register(email: %q, password: %q, name: %q)
	}`, email, password, name)
	result := graphqlRequest(query)
	data := result["data"].(map[string]interface{})
	return data["register"].(string)
}

func login(email, password string) string {
	query := fmt.Sprintf(`mutation {
		login(email: %q, password: %q)
	}`, email, password)
	result := graphqlRequest(query)
	data := result["data"].(map[string]interface{})
	return data["login"].(string)
}
