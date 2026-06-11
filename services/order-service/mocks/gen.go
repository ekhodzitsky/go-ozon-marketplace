package mocks

//go:generate mockgen -source=../internal/repository/repository.go -package=mocks -destination=mock_order_repository.go
//go:generate mockgen -source=../internal/saga/interfaces.go -package=mocks -destination=mock_saga_clients.go
//go:generate mockgen -source=../internal/usecase/interfaces.go -package=mocks -destination=mock_usecase.go
