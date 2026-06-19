//go:build chaos

package chaos

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestRateLimiterWithoutRedis(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}
	dockerComposeUp(t)

	// Stop Redis
	dockerStop(t, containerName("redis"))
	time.Sleep(2 * time.Second)

	gatewayURL := "http://localhost:8080/query"

	// Request without auth — gateway should return 401 (or similar), never 429
	resp := graphqlRequest(t, gatewayURL, `query { me { id } }`)
	if resp["_http_status"] == 429 {
		t.Fatal("rate limiter should fail open when redis is down, got 429")
	}

	// Request with valid auth token
	claims := auth.CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.New().String(),
			Issuer:    "go-ozon-marketplace",
			Audience:  jwt.ClaimStrings{"api-gateway"},
			ID:        uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Role: string(auth.RoleUser),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(jwtSecret))

	body, _ := json.Marshal(map[string]string{"query": `query { me { id } }`})
	req, err := http.NewRequest(http.MethodPost, gatewayURL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	httpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusTooManyRequests {
		t.Fatal("rate limiter should fail open when redis is down, got 429")
	}

	t.Logf("redis down response status: %d (expected not 429)", httpResp.StatusCode)
}
