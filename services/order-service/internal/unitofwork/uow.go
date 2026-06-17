package unitofwork

import "github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"

type UnitOfWork interface {
	OrderRepo() repository.OrderRepository
	OutboxRepo() repository.OutboxRepository
}
