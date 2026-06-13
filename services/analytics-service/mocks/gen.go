package mocks

//go:generate mockgen -package=mocks -destination=mock_analytics_usecase.go github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/usecase AnalyticsUsecase
//go:generate mockgen -package=mocks -destination=mock_event_repository.go github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/usecase EventRepository
