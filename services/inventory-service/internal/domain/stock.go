package domain

import "github.com/google/uuid"

type Stock struct {
	ProductID uuid.UUID
	Available int
	Reserved  int
}
