package mocks

//go:generate mockgen -source=../internal/saga/interfaces.go -package=mocks -destination=mock_saga_clients.go
//go:generate mockgen -source=../internal/saga/store/store.go -package=mocks -destination=mock_saga_store.go
//go:generate mockgen -source=../internal/saga/steps/step.go -package=mocks -destination=mock_saga_step.go
//go:generate mockgen -source=../internal/saga/statemachine/statemachine.go -package=mocks -destination=mock_saga_statemachine.go
//go:generate mockgen -source=../internal/saga/executor/executor.go -package=mocks -destination=mock_saga_executor.go
//go:generate mockgen -source=../internal/saga/compensator/compensator.go -package=mocks -destination=mock_saga_compensator.go
//go:generate mockgen -package=mocks -destination=mock_catalog_service_client.go github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1 CatalogServiceClient
//go:generate mockgen -package=mocks -destination=mock_inventory_service_client.go github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1 InventoryServiceClient
//go:generate mockgen -package=mocks -destination=mock_payment_service_client.go github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1 PaymentServiceClient
//go:generate mockgen -source=../internal/repository/repository.go -package=mocks -destination=mock_order_repository.go
//go:generate mockgen -source=../internal/unitofwork/uow.go -package=mocks -destination=mock_unitofwork.go
//go:generate mockgen -source=../internal/usecase/interfaces.go -package=mocks -destination=mock_usecase.go
