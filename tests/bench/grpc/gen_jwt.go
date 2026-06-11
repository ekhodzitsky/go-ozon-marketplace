//go:build ignore

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
)

func main() {
	secret := flag.String("secret", "", "JWT secret")
	userID := flag.String("user-id", "550e8400-e29b-41d4-a716-446655440000", "User ID")
	role := flag.String("role", "user", "Role")
	flag.Parse()

	if *secret == "" {
		*secret = os.Getenv("JWT_SECRET")
	}
	if *secret == "" {
		fmt.Fprintln(os.Stderr, "JWT_SECRET is required")
		os.Exit(1)
	}

	token, err := auth.GenerateToken(*secret, *userID, *role, time.Hour)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate token: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(token)
}
