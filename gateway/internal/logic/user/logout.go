package user

import (
	"context"
	"errors"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	"meshcart/gateway/internal/auth"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type LogoutLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LogoutLogic) Logout(req *types.UserLogoutRequest, identity *middleware.AuthIdentity) *common.BizError {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.user.logout", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "user"), attribute.String("biz.action", "logout"))

	if identity == nil || identity.SessionID == "" {
		return common.ErrUnauthorized
	}
	if req != nil && req.SessionID != "" && req.SessionID != identity.SessionID {
		return common.ErrForbidden
	}
	if l.svcCtx.SessionStore == nil {
		logx.L(ctx).Error("session store missing")
		return common.ErrInternalError
	}

	storeCtx, cancel := context.WithTimeout(ctx, l.svcCtx.Config.AuthSession.StoreTimeout)
	defer cancel()
	err := l.svcCtx.SessionStore.Delete(storeCtx, identity.SessionID)
	if err != nil && !errors.Is(err, auth.ErrSessionNotFound) {
		span.RecordError(err)
		logx.L(ctx).Error("delete auth session failed", zap.Error(err), zap.String("session_id", identity.SessionID))
		return common.ErrInternalError
	}

	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return nil
}
