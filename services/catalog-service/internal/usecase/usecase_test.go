package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/unitofwork"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/usecase"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type fakeUoW struct {
	beginErr       error
	commitErr      error
	rollbackErr    error
	productRepo    repository.ProductRepository
	outboxRepo     repository.OutboxRepository
	beginCalled    bool
	commitCalled   bool
	rollbackCalled bool
}

func (f *fakeUoW) Begin(ctx context.Context) error {
	f.beginCalled = true
	return f.beginErr
}

func (f *fakeUoW) Commit(ctx context.Context) error {
	f.commitCalled = true
	return f.commitErr
}

func (f *fakeUoW) Rollback(ctx context.Context) error {
	f.rollbackCalled = true
	return f.rollbackErr
}

func (f *fakeUoW) ProductRepo() repository.ProductRepository { return f.productRepo }
func (f *fakeUoW) OutboxRepo() repository.OutboxRepository   { return f.outboxRepo }

type testDeps struct {
	ctrl        *gomock.Controller
	productRepo *mocks.MockProductRepository
	searchRepo  *mocks.MockProductSearchRepository
	outboxRepo  *mocks.MockOutboxRepository
	uow         *fakeUoW
	uc          usecase.CatalogUsecase
}

func newTestDeps(t *testing.T) *testDeps {
	ctrl := gomock.NewController(t)
	productRepo := mocks.NewMockProductRepository(ctrl)
	searchRepo := mocks.NewMockProductSearchRepository(ctrl)
	outboxRepo := mocks.NewMockOutboxRepository(ctrl)
	uow := &fakeUoW{productRepo: productRepo, outboxRepo: outboxRepo}
	uc := usecase.NewCatalogUsecase(
		func() unitofwork.UnitOfWork { return uow },
		productRepo,
		searchRepo,
		100*time.Millisecond,
		100*time.Millisecond,
	)
	return &testDeps{ctrl: ctrl, productRepo: productRepo, searchRepo: searchRepo, outboxRepo: outboxRepo, uow: uow, uc: uc}
}

func TestCatalogUsecase_CreateProduct_Success(t *testing.T) {
	d := newTestDeps(t)
	existingID := uuid.New()
	d.productRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(existingID, nil)
	d.outboxRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	id, err := d.uc.CreateProduct(context.Background(), "Name", "Desc", 1000, []string{"cat"}, "key")
	require.NoError(t, err)
	assert.Equal(t, existingID, id)
	assert.True(t, d.uow.beginCalled)
	assert.True(t, d.uow.commitCalled)
}

func TestCatalogUsecase_CreateProduct_IdempotencyKeyReuse(t *testing.T) {
	d := newTestDeps(t)
	existingID := uuid.New()
	d.productRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(existingID, apperrors.ErrAlreadyExists)

	id, err := d.uc.CreateProduct(context.Background(), "Name", "Desc", 1000, []string{"cat"}, "key")
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrAlreadyExists))
	assert.Equal(t, existingID, id)
	assert.True(t, d.uow.beginCalled)
	assert.False(t, d.uow.commitCalled)
}

func TestCatalogUsecase_CreateProduct_BeginError(t *testing.T) {
	d := newTestDeps(t)
	d.uow.beginErr = errors.New("begin failed")

	_, err := d.uc.CreateProduct(context.Background(), "Name", "Desc", 1000, []string{"cat"}, "key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "begin uow")
}

func TestCatalogUsecase_CreateProduct_CreateError(t *testing.T) {
	d := newTestDeps(t)
	d.productRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(uuid.Nil, errors.New("db down"))

	_, err := d.uc.CreateProduct(context.Background(), "Name", "Desc", 1000, []string{"cat"}, "key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create product")
	assert.True(t, d.uow.rollbackCalled)
}

func TestCatalogUsecase_CreateProduct_OutboxError(t *testing.T) {
	d := newTestDeps(t)
	d.productRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(uuid.New(), nil)
	d.outboxRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("outbox down"))

	_, err := d.uc.CreateProduct(context.Background(), "Name", "Desc", 1000, []string{"cat"}, "key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create outbox event")
	assert.True(t, d.uow.rollbackCalled)
}

func TestCatalogUsecase_CreateProduct_CommitError(t *testing.T) {
	d := newTestDeps(t)
	d.productRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(uuid.New(), nil)
	d.outboxRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	d.uow.commitErr = errors.New("commit failed")

	_, err := d.uc.CreateProduct(context.Background(), "Name", "Desc", 1000, []string{"cat"}, "key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "commit uow")
}

func TestCatalogUsecase_CreateProduct_RollbackErrorOverridesNilError(t *testing.T) {
	d := newTestDeps(t)
	d.productRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(uuid.Nil, errors.New("create failed"))
	d.uow.rollbackErr = errors.New("rollback failed")

	_, err := d.uc.CreateProduct(context.Background(), "Name", "Desc", 1000, []string{"cat"}, "key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create product")
}

func TestCatalogUsecase_GetProduct_Success(t *testing.T) {
	d := newTestDeps(t)
	id := uuid.New()
	expected := &domain.Product{ID: id, Name: "P"}
	d.productRepo.EXPECT().GetByID(gomock.Any(), id).Return(expected, nil)

	product, err := d.uc.GetProduct(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, expected, product)
}

func TestCatalogUsecase_GetProduct_NotFound(t *testing.T) {
	d := newTestDeps(t)
	id := uuid.New()
	d.productRepo.EXPECT().GetByID(gomock.Any(), id).Return(nil, apperrors.ErrNotFound)

	_, err := d.uc.GetProduct(context.Background(), id)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrNotFound))
}

func TestCatalogUsecase_UpdateProduct_Success(t *testing.T) {
	d := newTestDeps(t)
	id := uuid.New()
	existing := &domain.Product{ID: id, Name: "Old", Description: "OldDesc", Price: 1000, Categories: []string{"old"}}
	d.productRepo.EXPECT().GetByID(gomock.Any(), id).Return(existing, nil)
	d.productRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	d.outboxRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	err := d.uc.UpdateProduct(context.Background(), id, "New", "NewDesc", 2000, []string{"new"})
	require.NoError(t, err)
	assert.True(t, d.uow.commitCalled)
}

func TestCatalogUsecase_UpdateProduct_PartialFields(t *testing.T) {
	d := newTestDeps(t)
	id := uuid.New()
	existing := &domain.Product{ID: id, Name: "Old", Description: "OldDesc", Price: 1000, Categories: []string{"old"}}
	d.productRepo.EXPECT().GetByID(gomock.Any(), id).Return(existing, nil)
	d.productRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	d.outboxRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	err := d.uc.UpdateProduct(context.Background(), id, "", "", 0, nil)
	require.NoError(t, err)
	assert.Equal(t, "Old", existing.Name)
	assert.Equal(t, int64(1000), existing.Price)
}

func TestCatalogUsecase_UpdateProduct_NotFoundOnGet(t *testing.T) {
	d := newTestDeps(t)
	id := uuid.New()
	d.productRepo.EXPECT().GetByID(gomock.Any(), id).Return(nil, apperrors.ErrNotFound)

	err := d.uc.UpdateProduct(context.Background(), id, "New", "", 0, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrNotFound))
}

func TestCatalogUsecase_UpdateProduct_RowsAffectedZero(t *testing.T) {
	d := newTestDeps(t)
	id := uuid.New()
	existing := &domain.Product{ID: id, Name: "Old"}
	d.productRepo.EXPECT().GetByID(gomock.Any(), id).Return(existing, nil)
	d.productRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(apperrors.ErrNotFound)

	err := d.uc.UpdateProduct(context.Background(), id, "New", "", 0, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrNotFound))
}

func TestCatalogUsecase_UpdateProduct_CommitError(t *testing.T) {
	d := newTestDeps(t)
	id := uuid.New()
	existing := &domain.Product{ID: id, Name: "Old"}
	d.productRepo.EXPECT().GetByID(gomock.Any(), id).Return(existing, nil)
	d.productRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	d.outboxRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	d.uow.commitErr = errors.New("commit failed")

	err := d.uc.UpdateProduct(context.Background(), id, "New", "", 0, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "commit uow")
}

func TestCatalogUsecase_DeleteProduct_Success(t *testing.T) {
	d := newTestDeps(t)
	id := uuid.New()
	d.productRepo.EXPECT().Delete(gomock.Any(), id).Return(nil)
	d.outboxRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	err := d.uc.DeleteProduct(context.Background(), id)
	require.NoError(t, err)
	assert.True(t, d.uow.commitCalled)
}

func TestCatalogUsecase_DeleteProduct_NotFound(t *testing.T) {
	d := newTestDeps(t)
	id := uuid.New()
	d.productRepo.EXPECT().Delete(gomock.Any(), id).Return(apperrors.ErrNotFound)

	err := d.uc.DeleteProduct(context.Background(), id)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrNotFound))
}

func TestCatalogUsecase_DeleteProduct_CommitError(t *testing.T) {
	d := newTestDeps(t)
	id := uuid.New()
	d.productRepo.EXPECT().Delete(gomock.Any(), id).Return(nil)
	d.outboxRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	d.uow.commitErr = errors.New("commit failed")

	err := d.uc.DeleteProduct(context.Background(), id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "commit uow")
}

func TestCatalogUsecase_ListProducts(t *testing.T) {
	d := newTestDeps(t)
	expected := []*domain.Product{{ID: uuid.New(), Name: "P"}}
	d.productRepo.EXPECT().List(gomock.Any(), 1, 10).Return(expected, 1, nil)

	products, total, err := d.uc.ListProducts(context.Background(), 1, 10)
	require.NoError(t, err)
	assert.Equal(t, expected, products)
	assert.Equal(t, 1, total)
}

func TestCatalogUsecase_SearchProducts(t *testing.T) {
	d := newTestDeps(t)
	expected := []*domain.Product{{ID: uuid.New(), Name: "P"}}
	d.searchRepo.EXPECT().Search(gomock.Any(), "query", 1, 10).Return(expected, 1, nil)

	products, total, err := d.uc.SearchProducts(context.Background(), "query", 1, 10)
	require.NoError(t, err)
	assert.Equal(t, expected, products)
	assert.Equal(t, 1, total)
}
