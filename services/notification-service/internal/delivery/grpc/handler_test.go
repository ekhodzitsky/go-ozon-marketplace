package grpc_test

import (
	"context"
	"testing"

	notificationv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/notification/v1"
	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/mocks"
	"github.com/ekhodzitsky/go-ozon-marketplace/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func authCtxWithRole(role middleware.Role) context.Context {
	return context.WithValue(context.Background(), middleware.ContextKeyRole, string(role))
}

func TestNotificationHandler_SendEmail(t *testing.T) {
	t.Parallel()

	testsCases := []struct {
		name      string
		ctx       context.Context
		req       *notificationv1.SendEmailRequest
		setupMock func(ctrl *gomock.Controller) *mocks.MockNotificationUsecase
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success",
			ctx:  authCtxWithRole(middleware.RoleService),
			req:  tests.NewSendEmailRequestBuilder().WithTo("a@b.c").WithSubject("S").WithBody("B").Build(),
			setupMock: func(ctrl *gomock.Controller) *mocks.MockNotificationUsecase {
				m := mocks.NewMockNotificationUsecase(ctrl)
				m.EXPECT().SendEmail(gomock.Any(), "a@b.c", "S", "B").Return(nil)
				return m
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name:     "missing_role",
			ctx:      context.Background(),
			req:      tests.NewSendEmailRequestBuilder().WithTo("a@b.c").WithSubject("S").WithBody("B").Build(),
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name:     "external_denied",
			ctx:      authCtxWithRole(middleware.RoleUser),
			req:      tests.NewSendEmailRequestBuilder().WithTo("a@b.c").WithSubject("S").WithBody("B").Build(),
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name: "usecase_not_found",
			ctx:  authCtxWithRole(middleware.RoleService),
			req:  tests.NewSendEmailRequestBuilder().WithTo("a@b.c").WithSubject("S").WithBody("B").Build(),
			setupMock: func(ctrl *gomock.Controller) *mocks.MockNotificationUsecase {
				m := mocks.NewMockNotificationUsecase(ctrl)
				m.EXPECT().SendEmail(gomock.Any(), "a@b.c", "S", "B").Return(apperrors.ErrNotFound)
				return m
			},
			wantCode: codes.NotFound,
			wantErr:  true,
		},
		{
			name: "usecase_invalid_argument",
			ctx:  authCtxWithRole(middleware.RoleService),
			req:  tests.NewSendEmailRequestBuilder().WithTo("a@b.c").WithSubject("S").WithBody("B").Build(),
			setupMock: func(ctrl *gomock.Controller) *mocks.MockNotificationUsecase {
				m := mocks.NewMockNotificationUsecase(ctrl)
				m.EXPECT().SendEmail(gomock.Any(), "a@b.c", "S", "B").Return(apperrors.ErrInvalidArgument)
				return m
			},
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name: "usecase_generic_error",
			ctx:  authCtxWithRole(middleware.RoleService),
			req:  tests.NewSendEmailRequestBuilder().WithTo("a@b.c").WithSubject("S").WithBody("B").Build(),
			setupMock: func(ctrl *gomock.Controller) *mocks.MockNotificationUsecase {
				m := mocks.NewMockNotificationUsecase(ctrl)
				m.EXPECT().SendEmail(gomock.Any(), "a@b.c", "S", "B").Return(assert.AnError)
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

			mockUC := mocks.NewMockNotificationUsecase(ctrl)
			if tt.setupMock != nil {
				mockUC = tt.setupMock(ctrl)
			}
			h := grpcdelivery.NewNotificationHandler(mockUC)
			_, err := h.SendEmail(tt.ctx, tt.req)

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
