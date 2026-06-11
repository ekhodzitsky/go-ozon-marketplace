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
	"github.com/ekhodzitsky/go-ozon-marketplace/tests"
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
			req:  tests.NewCreateProductRequestBuilder().WithName("P").WithDescription("D").WithPrice(10.0).WithCategories([]string{"c"}).Build(),
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				m := mocks.NewMockCatalogUsecase(ctrl)
				m.EXPECT().CreateProduct(gomock.Any(), "P", "D", int64(1000), []string{"c"}).Return(uuid.New(), nil)
				return m
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name: "usecase_error",
			req:  tests.NewCreateProductRequestBuilder().WithName("P").WithPrice(10.0).Build(),
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				m := mocks.NewMockCatalogUsecase(ctrl)
				m.EXPECT().CreateProduct(gomock.Any(), "P", "", int64(1000), gomock.Any()).Return(uuid.Nil, errors.New("boom"))
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
			req:  tests.NewGetProductRequestBuilder().WithProductID(productID.String()).Build(),
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
			req:     tests.NewGetProductRequestBuilder().WithProductID("bad").Build(),
			wantErr: true,
		},
		{
			name: "not_found",
			req:  tests.NewGetProductRequestBuilder().WithProductID(productID.String()).Build(),
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
			req:  tests.NewGetProductRequestBuilder().WithProductID(productID.String()).Build(),
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
			req:  tests.NewListProductsRequestBuilder().Build(),
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
			req:  tests.NewListProductsRequestBuilder().Build(),
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
			req:  tests.NewSearchProductsRequestBuilder().WithQuery("q").Build(),
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
			req:  tests.NewSearchProductsRequestBuilder().WithQuery("q").Build(),
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
			req:  tests.NewUpdateProductRequestBuilder().WithProductID(productID.String()).WithName("NewName").WithPrice(20.0).Build(),
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				m := mocks.NewMockCatalogUsecase(ctrl)
				m.EXPECT().UpdateProduct(gomock.Any(), productID, "NewName", "", int64(2000), gomock.Any()).Return(nil)
				return m
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name:     "missing_role",
			ctx:      context.Background(),
			req:      tests.NewUpdateProductRequestBuilder().WithProductID(productID.String()).WithName("NewName").Build(),
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name:     "invalid_product_id",
			ctx:      authCtxWithRole(middleware.RoleAdmin),
			req:      tests.NewUpdateProductRequestBuilder().WithProductID("bad").WithName("NewName").Build(),
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name: "not_found",
			ctx:  authCtxWithRole(middleware.RoleAdmin),
			req:  tests.NewUpdateProductRequestBuilder().WithProductID(productID.String()).WithName("NewName").Build(),
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
			req:  tests.NewUpdateProductRequestBuilder().WithProductID(productID.String()).WithName("NewName").Build(),
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
			req:  tests.NewDeleteProductRequestBuilder().WithProductID(productID.String()).Build(),
			setupMock: func(ctrl *gomock.Controller) *mocks.MockCatalogUsecase {
				m := mocks.NewMockCatalogUsecase(ctrl)
				m.EXPECT().DeleteProduct(gomock.Any(), productID).Return(nil)
				return m
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name:     "missing_role",
			ctx:      context.Background(),
			req:      tests.NewDeleteProductRequestBuilder().WithProductID(productID.String()).Build(),
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name:     "invalid_product_id",
			ctx:      authCtxWithRole(middleware.RoleAdmin),
			req:      tests.NewDeleteProductRequestBuilder().WithProductID("bad").Build(),
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name: "not_found",
			ctx:  authCtxWithRole(middleware.RoleAdmin),
			req:  tests.NewDeleteProductRequestBuilder().WithProductID(productID.String()).Build(),
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
			req:  tests.NewDeleteProductRequestBuilder().WithProductID(productID.String()).Build(),
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