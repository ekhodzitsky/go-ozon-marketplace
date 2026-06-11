package mocks

//go:generate mockgen -source=../internal/repository/repository.go -package=mocks -destination=mock_inventory_repository.go
//go:generate mockgen -source=../internal/usecase/interfaces.go -package=mocks -destination=mock_inventory_usecase.go
