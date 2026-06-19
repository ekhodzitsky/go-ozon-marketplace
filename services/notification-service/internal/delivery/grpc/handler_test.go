package grpc_test

import (
	"context"
	"testing"

	notificationv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/notification/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func authCtxWithRole(role auth.Role) context.Context {
	return context.WithValue(context.Background(), auth.ContextKeyRole, string(role))
}

func newSendEmailRequest(to, subject, body string) *notificationv1.SendEmailRequest {
	return &notificationv1.SendEmailRequest{
		To:      to,
		Subject: subject,
		Body:    body,
	}
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
			ctx:  authCtxWithRole(auth.RoleService),
			req:  newSendEmailRequest("a@b.c", "S", "B"),
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
			req:      newSendEmailRequest("a@b.c", "S", "B"),
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name:     "external_denied",
			ctx:      authCtxWithRole(auth.RoleUser),
			req:      newSendEmailRequest("a@b.c", "S", "B"),
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name:     "invalid_email",
			ctx:      authCtxWithRole(auth.RoleService),
			req:      newSendEmailRequest("not-an-email", "S", "B"),
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name:     "missing_subject",
			ctx:      authCtxWithRole(auth.RoleService),
			req:      newSendEmailRequest("a@b.c", "", "B"),
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name:     "missing_body",
			ctx:      authCtxWithRole(auth.RoleService),
			req:      newSendEmailRequest("a@b.c", "S", ""),
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name: "usecase_not_found",
			ctx:  authCtxWithRole(auth.RoleService),
			req:  newSendEmailRequest("a@b.c", "S", "B"),
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
			ctx:  authCtxWithRole(auth.RoleService),
			req:  newSendEmailRequest("a@b.c", "S", "B"),
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
			ctx:  authCtxWithRole(auth.RoleService),
			req:  newSendEmailRequest("a@b.c", "S", "B"),
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
