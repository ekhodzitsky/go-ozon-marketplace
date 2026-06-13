package mocks

//go:generate mockgen -source=../internal/infrastructure/grpcclient/catalog.go -package=mocks -destination=mock_catalog_client.go
//go:generate mockgen -source=../internal/repository/repository.go -package=mocks -destination=mock_order_repository.go
//go:generate mockgen -source=../internal/saga/interfaces.go -package=mocks -destination=mock_saga_clients.go
//go:generate mockgen -source=../internal/unitofwork/uow.go -package=mocks -destination=mock_unitofwork.go
//go:generate mockgen -source=../internal/usecase/interfaces.go -package=mocks -destination=mock_usecase.go
