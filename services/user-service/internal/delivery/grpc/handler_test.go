package grpc_test

import (
	"context"
	"testing"
	"time"

	userv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/user/v1"
	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/mocks"
	"github.com/ekhodzitsky/go-ozon-marketplace/tests"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestUserHandler_Register(t *testing.T) {
	t.Parallel()

	testsCases := []struct {
		name    string
		req     *userv1.RegisterRequest
		mock    func(m *mocks.MockUserUsecase)
		wantErr bool
		wantID  bool
	}{
		{
			name: "success",
			req:  tests.NewUserRequestBuilder().WithEmail("a@b.co").WithPassword("password123").WithName("NN").BuildRegister(),
			mock: func(m *mocks.MockUserUsecase) {
				m.EXPECT().Register(gomock.Any(), "a@b.co", "password123", "NN").Return(uuid.MustParse("11111111-1111-1111-1111-111111111111"), nil)
			},
			wantErr: false,
			wantID:  true,
		},
		{
			name: "usecase_error",
			req:  tests.NewUserRequestBuilder().WithEmail("a@b.co").WithPassword("password123").WithName("NN").BuildRegister(),
			mock: func(m *mocks.MockUserUsecase) {
				m.EXPECT().Register(gomock.Any(), "a@b.co", "password123", "NN").Return(uuid.Nil, assert.AnError)
			},
			wantErr: true,
		},
	}

	for _, tt := range testsCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockUC := mocks.NewMockUserUsecase(ctrl)
			if tt.mock != nil {
				tt.mock(mockUC)
			}
			h := grpcdelivery.NewUserHandler(mockUC)
			resp, err := h.Register(context.Background(), tt.req)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.wantID {
				assert.NotEmpty(t, resp.UserId)
			}
		})
	}
}

func TestUserHandler_Login(t *testing.T) {
	t.Parallel()

	testsCases := []struct {
		name      string
		req       *userv1.LoginRequest
		mock      func(m *mocks.MockUserUsecase)
		wantErr   bool
		wantToken bool
	}{
		{
			name: "success",
			req:  tests.NewUserRequestBuilder().WithEmail("a@b.co").WithPassword("password123").BuildLogin(),
			mock: func(m *mocks.MockUserUsecase) {
				m.EXPECT().Login(gomock.Any(), "a@b.co", "password123").Return("token", nil)
			},
			wantErr:   false,
			wantToken: true,
		},
		{
			name: "usecase_error",
			req:  tests.NewUserRequestBuilder().WithEmail("a@b.co").WithPassword("password123").BuildLogin(),
			mock: func(m *mocks.MockUserUsecase) {
				m.EXPECT().Login(gomock.Any(), "a@b.co", "password123").Return("", assert.AnError)
			},
			wantErr: true,
		},
	}

	for _, tt := range testsCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockUC := mocks.NewMockUserUsecase(ctrl)
			if tt.mock != nil {
				tt.mock(mockUC)
			}
			h := grpcdelivery.NewUserHandler(mockUC)
			resp, err := h.Login(context.Background(), tt.req)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.wantToken {
				assert.Equal(t, "token", resp.Token)
			}
		})
	}
}

func TestUserHandler_GetUser(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	now := time.Now().UTC()

	testsCases := []struct {
		name      string
		req       *userv1.GetUserRequest
		mock      func(m *mocks.MockUserUsecase)
		wantCode  codes.Code
		wantErr   bool
		wantEmail string
	}{
		{
			name: "success",
			req:  tests.NewGetUserRequestBuilder().WithUserID(userID.String()).Build(),
			mock: func(m *mocks.MockUserUsecase) {
				m.EXPECT().GetUser(gomock.Any(), userID).Return(&domain.User{
					ID:        userID,
					Email:     "a@b.c",
					Name:      "N",
					CreatedAt: now,
				}, nil)
			},
			wantCode:  codes.OK,
			wantErr:   false,
			wantEmail: "a@b.c",
		},
		{
			name:    "invalid_uuid",
			req:     tests.NewGetUserRequestBuilder().WithUserID("bad").Build(),
			wantErr: true,
		},
		{
			name: "not_found",
			req:  tests.NewGetUserRequestBuilder().WithUserID(userID.String()).Build(),
			mock: func(m *mocks.MockUserUsecase) {
				m.EXPECT().GetUser(gomock.Any(), userID).Return(nil, apperrors.ErrNotFound)
			},
			wantCode: codes.NotFound,
			wantErr:  true,
		},
		{
			name: "internal_error",
			req:  tests.NewGetUserRequestBuilder().WithUserID(userID.String()).Build(),
			mock: func(m *mocks.MockUserUsecase) {
				m.EXPECT().GetUser(gomock.Any(), userID).Return(nil, assert.AnError)
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

			mockUC := mocks.NewMockUserUsecase(ctrl)
			if tt.mock != nil {
				tt.mock(mockUC)
			}
			h := grpcdelivery.NewUserHandler(mockUC)
			resp, err := h.GetUser(context.Background(), tt.req)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantCode != codes.OK {
					s, ok := status.FromError(err)
					require.True(t, ok, "expected gRPC status error")
					assert.Equal(t, tt.wantCode, s.Code())
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantEmail, resp.Email)
			assert.Equal(t, "N", resp.Name)
			assert.Equal(t, userID.String(), resp.UserId)
		})
	}
}
