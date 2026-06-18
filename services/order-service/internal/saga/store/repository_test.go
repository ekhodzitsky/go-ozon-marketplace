package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/store"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRepositoryStore_Create_Delegates(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockSagaRepository(ctrl)
	s := store.NewRepositoryStore(repo)
	saga := &domain.Saga{ID: uuid.New()}

	repo.EXPECT().Create(gomock.Any(), saga).Return(nil)

	err := s.Create(context.Background(), saga)
	require.NoError(t, err)
}

func TestRepositoryStore_Create_PropagatesError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockSagaRepository(ctrl)
	s := store.NewRepositoryStore(repo)
	saga := &domain.Saga{ID: uuid.New()}
	want := errors.New("db unavailable")

	repo.EXPECT().Create(gomock.Any(), saga).Return(want)

	err := s.Create(context.Background(), saga)
	assert.ErrorIs(t, err, want)
}

func TestRepositoryStore_GetByOrderID_Delegates(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockSagaRepository(ctrl)
	s := store.NewRepositoryStore(repo)
	orderID := uuid.New()
	want := &domain.Saga{ID: uuid.New(), OrderID: orderID}

	repo.EXPECT().GetByOrderID(gomock.Any(), orderID).Return(want, nil)

	got, err := s.GetByOrderID(context.Background(), orderID)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestRepositoryStore_Save_Delegates(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockSagaRepository(ctrl)
	s := store.NewRepositoryStore(repo)
	saga := &domain.Saga{ID: uuid.New()}

	repo.EXPECT().Save(gomock.Any(), saga).Return(nil)

	err := s.Save(context.Background(), saga)
	require.NoError(t, err)
}

func TestRepositoryStore_ListIncomplete_Delegates(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockSagaRepository(ctrl)
	s := store.NewRepositoryStore(repo)
	want := []domain.Saga{{ID: uuid.New()}}

	repo.EXPECT().ListIncomplete(gomock.Any(), 10).Return(want, nil)

	got, err := s.ListIncomplete(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
