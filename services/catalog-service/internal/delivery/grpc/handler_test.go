package grpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCatalogHandler_CreateProduct(t *testing.T) {
	t.Parallel()

	testsCases := []struct {
		name      string
		req       *catalogv1.CreateProductRequest
		setupMock func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success",
			req: &catalogv1.CreateProductRequest{
				Name:           "PP",
				Description:    "D",
				PriceCents:     1000,
				Categories:     []string{"c"},
				IdempotencyKey: "key-1",
			},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				m := mocks.NewMockCatalogUsecase(ctrl)
				m.EXPECT().CreateProduct(gomock.Any(), "PP", "D", int64(1000), []string{"c"}, "key-1").Return(uuid.New(), nil)
				return m
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name: "missing_idempotency_key",
			req: &catalogv1.CreateProductRequest{
				Name:       "PP",
				PriceCents: 1000,
			},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				return mocks.NewMockCatalogUsecase(ctrl)
			},
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name: "usecase_error",
			req: &catalogv1.CreateProductRequest{
				Name:           "PP",
				PriceCents:     1000,
				IdempotencyKey: "key-2",
			},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				m := mocks.NewMockCatalogUsecase(ctrl)
				m.EXPECT().CreateProduct(gomock.Any(), "PP", "", int64(1000), gomock.Any(), "key-2").Return(uuid.Nil, errors.New("boom"))
				return m
			},
			wantCode: codes.Unknown,
			wantErr:  true,
		},
		{
			name: "already_exists",
			req: &catalogv1.CreateProductRequest{
				Name:           "PP",
				PriceCents:     1000,
				IdempotencyKey: "key-3",
			},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				m := mocks.NewMockCatalogUsecase(ctrl)
				m.EXPECT().CreateProduct(gomock.Any(), "PP", "", int64(1000), gomock.Any(), "key-3").Return(uuid.Nil, apperrors.ErrAlreadyExists)
				return m
			},
			wantCode: codes.AlreadyExists,
			wantErr:  true,
		},
	}

	for _, tt := range testsCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockUC := tt.setupMock(ctrl)
			h := grpcdelivery.NewCatalogHandler(mockUC)
			_, err := h.CreateProduct(context.Background(), tt.req)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantCode != codes.OK {
					s, _ := status.FromError(err)
					assert.Equal(t, tt.wantCode, s.Code())
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestCatalogHandler_GetProduct(t *testing.T) {
	t.Parallel()

	productID := uuid.New()

	testsCases := []struct {
		name      string
		req       *catalogv1.GetProductRequest
		setupMock func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success",
			req:  &catalogv1.GetProductRequest{ProductId: productID.String()},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				m := mocks.NewMockCatalogUsecase(ctrl)
				m.EXPECT().GetProduct(gomock.Any(), productID).Return(&domain.Product{ID: productID, Name: "P", CreatedAt: time.Now().UTC()}, nil)
				return m
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name:    "invalid_uuid",
			req:     &catalogv1.GetProductRequest{ProductId: "bad"},
			wantErr: true,
		},
		{
			name: "not_found",
			req:  &catalogv1.GetProductRequest{ProductId: productID.String()},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				m := mocks.NewMockCatalogUsecase(ctrl)
				m.EXPECT().GetProduct(gomock.Any(), productID).Return(nil, apperrors.ErrNotFound)
				return m
			},
			wantCode: codes.NotFound,
			wantErr:  true,
		},
		{
			name: "internal_error",
			req:  &catalogv1.GetProductRequest{ProductId: productID.String()},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				m := mocks.NewMockCatalogUsecase(ctrl)
				m.EXPECT().GetProduct(gomock.Any(), productID).Return(nil, errors.New("db down"))
				return m
			},
			wantCode: codes.Internal,
			wantErr:  true,
		},
	}

	for _, tt := range testsCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockUC := mocks.NewMockCatalogUsecase(ctrl)
			if tt.setupMock != nil {
				mockUC = tt.setupMock(ctrl)
			}
			h := grpcdelivery.NewCatalogHandler(mockUC)
			_, err := h.GetProduct(context.Background(), tt.req)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantCode != codes.OK {
					s, _ := status.FromError(err)
					assert.Equal(t, tt.wantCode, s.Code())
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestCatalogHandler_ListProducts(t *testing.T) {
	t.Parallel()

	testsCases := []struct {
		name      string
		req       *catalogv1.ListProductsRequest
		setupMock func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success",
			req:  &catalogv1.ListProductsRequest{Page: 1, PageSize: 10},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				m := mocks.NewMockCatalogUsecase(ctrl)
				m.EXPECT().ListProducts(gomock.Any(), 1, 10).Return([]*domain.Product{}, 0, nil)
				return m
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name: "usecase_error",
			req:  &catalogv1.ListProductsRequest{Page: 1, PageSize: 10},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				m := mocks.NewMockCatalogUsecase(ctrl)
				m.EXPECT().ListProducts(gomock.Any(), 1, 10).Return(nil, 0, errors.New("boom"))
				return m
			},
			wantCode: codes.Unknown,
			wantErr:  true,
		},
	}

	for _, tt := range testsCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockUC := tt.setupMock(ctrl)
			h := grpcdelivery.NewCatalogHandler(mockUC)
			_, err := h.ListProducts(context.Background(), tt.req)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantCode != codes.OK {
					s, _ := status.FromError(err)
					assert.Equal(t, tt.wantCode, s.Code())
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestCatalogHandler_SearchProducts(t *testing.T) {
	t.Parallel()

	testsCases := []struct {
		name      string
		req       *catalogv1.SearchProductsRequest
		setupMock func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success",
			req:  &catalogv1.SearchProductsRequest{Query: "q", Page: 1, PageSize: 10},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				m := mocks.NewMockCatalogUsecase(ctrl)
				m.EXPECT().SearchProducts(gomock.Any(), "q", 1, 10).Return([]*domain.Product{}, 0, nil)
				return m
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name: "usecase_error",
			req:  &catalogv1.SearchProductsRequest{Query: "q", Page: 1, PageSize: 10},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				m := mocks.NewMockCatalogUsecase(ctrl)
				m.EXPECT().SearchProducts(gomock.Any(), "q", 1, 10).Return(nil, 0, errors.New("boom"))
				return m
			},
			wantCode: codes.Unknown,
			wantErr:  true,
		},
	}

	for _, tt := range testsCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockUC := tt.setupMock(ctrl)
			h := grpcdelivery.NewCatalogHandler(mockUC)
			_, err := h.SearchProducts(context.Background(), tt.req)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantCode != codes.OK {
					s, _ := status.FromError(err)
					assert.Equal(t, tt.wantCode, s.Code())
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func authCtxWithRole(role middleware.Role) context.Context {
	return context.WithValue(context.Background(), middleware.ContextKeyRole, string(role))
}

func TestCatalogHandler_UpdateProduct(t *testing.T) {
	t.Parallel()

	productID := uuid.New()

	testsCases := []struct {
		name      string
		ctx       context.Context
		req       *catalogv1.UpdateProductRequest
		setupMock func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success",
			ctx:  authCtxWithRole(middleware.RoleAdmin),
			req: &catalogv1.UpdateProductRequest{
				ProductId:  productID.String(),
				Name:       "NewName",
				PriceCents: 2000,
				Categories: []string{"c"},
			},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				m := mocks.NewMockCatalogUsecase(ctrl)
				m.EXPECT().UpdateProduct(gomock.Any(), productID, "NewName", "", int64(2000), []string{"c"}).Return(nil)
				return m
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name: "missing_role",
			ctx:  context.Background(),
			req: &catalogv1.UpdateProductRequest{
				ProductId: productID.String(),
				Name:      "NewName",
			},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				return mocks.NewMockCatalogUsecase(ctrl)
			},
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name: "invalid_product_id",
			ctx:  authCtxWithRole(middleware.RoleAdmin),
			req: &catalogv1.UpdateProductRequest{
				ProductId: "bad",
				Name:      "NewName",
			},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				return mocks.NewMockCatalogUsecase(ctrl)
			},
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name: "not_found",
			ctx:  authCtxWithRole(middleware.RoleAdmin),
			req: &catalogv1.UpdateProductRequest{
				ProductId: productID.String(),
				Name:      "NewName",
			},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				m := mocks.NewMockCatalogUsecase(ctrl)
				m.EXPECT().UpdateProduct(gomock.Any(), productID, "NewName", "", int64(0), gomock.Any()).Return(apperrors.ErrNotFound)
				return m
			},
			wantCode: codes.NotFound,
			wantErr:  true,
		},
		{
			name: "internal_error",
			ctx:  authCtxWithRole(middleware.RoleAdmin),
			req: &catalogv1.UpdateProductRequest{
				ProductId: productID.String(),
				Name:      "NewName",
			},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				m := mocks.NewMockCatalogUsecase(ctrl)
				m.EXPECT().UpdateProduct(gomock.Any(), productID, "NewName", "", int64(0), gomock.Any()).Return(errors.New("boom"))
				return m
			},
			wantCode: codes.Internal,
			wantErr:  true,
		},
	}

	for _, tt := range testsCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockUC := mocks.NewMockCatalogUsecase(ctrl)
			if tt.setupMock != nil {
				mockUC = tt.setupMock(ctrl)
			}
			h := grpcdelivery.NewCatalogHandler(mockUC)
			_, err := h.UpdateProduct(tt.ctx, tt.req)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantCode != codes.OK {
					s, _ := status.FromError(err)
					assert.Equal(t, tt.wantCode, s.Code())
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestCatalogHandler_DeleteProduct(t *testing.T) {
	t.Parallel()

	productID := uuid.New()

	testsCases := []struct {
		name      string
		ctx       context.Context
		req       *catalogv1.DeleteProductRequest
		setupMock func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success",
			ctx:  authCtxWithRole(middleware.RoleAdmin),
			req:  &catalogv1.DeleteProductRequest{ProductId: productID.String()},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				m := mocks.NewMockCatalogUsecase(ctrl)
				m.EXPECT().DeleteProduct(gomock.Any(), productID).Return(nil)
				return m
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name: "missing_role",
			ctx:  context.Background(),
			req:  &catalogv1.DeleteProductRequest{ProductId: productID.String()},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				return mocks.NewMockCatalogUsecase(ctrl)
			},
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name: "invalid_product_id",
			ctx:  authCtxWithRole(middleware.RoleAdmin),
			req:  &catalogv1.DeleteProductRequest{ProductId: "bad"},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				return mocks.NewMockCatalogUsecase(ctrl)
			},
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name: "not_found",
			ctx:  authCtxWithRole(middleware.RoleAdmin),
			req:  &catalogv1.DeleteProductRequest{ProductId: productID.String()},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				m := mocks.NewMockCatalogUsecase(ctrl)
				m.EXPECT().DeleteProduct(gomock.Any(), productID).Return(apperrors.ErrNotFound)
				return m
			},
			wantCode: codes.NotFound,
			wantErr:  true,
		},
		{
			name: "internal_error",
			ctx:  authCtxWithRole(middleware.RoleAdmin),
			req:  &catalogv1.DeleteProductRequest{ProductId: productID.String()},
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				m := mocks.NewMockCatalogUsecase(ctrl)
				m.EXPECT().DeleteProduct(gomock.Any(), productID).Return(errors.New("boom"))
				return m
			},
			wantCode: codes.Internal,
			wantErr:  true,
		},
	}

	for _, tt := range testsCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockUC := mocks.NewMockCatalogUsecase(ctrl)
			if tt.setupMock != nil {
				mockUC = tt.setupMock(ctrl)
			}
			h := grpcdelivery.NewCatalogHandler(mockUC)
			_, err := h.DeleteProduct(tt.ctx, tt.req)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantCode != codes.OK {
					s, _ := status.FromError(err)
					assert.Equal(t, tt.wantCode, s.Code())
				}
				return
			}
			require.NoError(t, err)
		})
	}
}
