package user

import (
	"context"
	"strings"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	"meshcart/gateway/internal/authz"
	"meshcart/gateway/internal/logic/logicutil"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	userrpc "meshcart/gateway/rpc/user"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type UpdateUserRoleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateUserRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUserRoleLogic {
	return &UpdateUserRoleLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateUserRoleLogic) Update(targetUserID int64, role string, identity *middleware.AuthIdentity) *common.BizError {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.user.update_role", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "user"), attribute.String("biz.action", "update_role"), attribute.Int64("target_user_id", targetUserID))

	if targetUserID <= 0 || strings.TrimSpace(role) == "" {
		return common.ErrInvalidParam
	}
	if identity == nil {
		return common.ErrUnauthorized
	}
	if !l.svcCtx.AccessControl.Enforce(identity.Role, "user", authz.ActionManageRole, 0, identity.UserID, 0) {
		return common.ErrForbidden
	}

	resp, err := l.svcCtx.UserClient.UpdateUserRole(ctx, &userrpc.UpdateUserRoleRequest{
		UserID: targetUserID,
		Role:   role,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "user rpc update role failed")
		logx.L(ctx).Error("user rpc update role failed", zap.Error(err))
		return logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		logx.L(ctx).Warn("user rpc update role returned business error",
			zap.Int64("target_user_id", targetUserID),
			zap.String("role", role),
			zap.Int32("code", resp.Code),
			zap.String("message", resp.Message),
		)
		return common.NewBizError(resp.Code, resp.Message)
	}

	span.SetStatus(codes.Ok, "ok")
	return nil
}
