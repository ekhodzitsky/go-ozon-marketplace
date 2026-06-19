package mocks

//go:generate mockgen -source=../internal/saga/steps.go -package=mocks -destination=mock_saga.go
//go:generate mockgen -source=../internal/saga/domain.go -package=mocks -destination=mock_saga_repository.go
//go:generate mockgen -package=mocks -destination=mock_catalog_service_client.go github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1 CatalogServiceClient
//go:generate mockgen -package=mocks -destination=mock_inventory_service_client.go github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1 InventoryServiceClient
//go:generate mockgen -package=mocks -destination=mock_payment_service_client.go github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1 PaymentServiceClient
//go:generate mockgen -source=../internal/repository/repository.go -package=mocks -destination=mock_order_repository.go
//go:generate mockgen -source=../internal/unitofwork/uow.go -package=mocks -destination=mock_unitofwork.go
//go:generate mockgen -source=../internal/usecase/interfaces.go -package=mocks -destination=mock_usecase.go
