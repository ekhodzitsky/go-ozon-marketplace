package validation

import (
	"fmt"
	"regexp"
	"unicode/utf8"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func ValidateEmail(email string) error {
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	return nil
}

func ValidateName(name string) error {
	n := utf8.RuneCountInString(name)
	if n < 2 || n > 100 {
		return fmt.Errorf("name must be between 2 and 100 characters")
	}
	return nil
}

func ValidatePrice(price float64) error {
	if price <= 0 {
		return fmt.Errorf("price must be greater than 0")
	}
	return nil
}

func ValidatePriceCents(cents int64) error {
	if cents <= 0 {
		return fmt.Errorf("price must be greater than 0")
	}
	return nil
}

func ValidateQuantity(qty int32) error {
	if qty <= 0 {
		return fmt.Errorf("quantity must be greater than 0")
	}
	return nil
}

func ValidatePageSize(ps int32) error {
	if ps < 1 || ps > 100 {
		return fmt.Errorf("page size must be between 1 and 100")
	}
	return nil
}
