package user

import (
	"context"
	"errors"
	"strings"
	"time"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	"meshcart/gateway/internal/auth"
	"meshcart/gateway/internal/logic/logicutil"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	userrpc "meshcart/gateway/rpc/user"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type RefreshTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefreshTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshTokenLogic {
	return &RefreshTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RefreshTokenLogic) Refresh(req *types.UserRefreshTokenRequest) (*types.UserRefreshTokenData, *common.BizError) {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.user.refresh_token", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "user"), attribute.String("biz.action", "refresh_token"))

	if req == nil || strings.TrimSpace(req.RefreshToken) == "" {
		span.SetAttributes(
			attribute.Bool("biz.success", false),
			attribute.String("biz.type", "business"),
			attribute.Int("biz.code", int(common.ErrInvalidParam.Code)),
			attribute.String("biz.message", common.ErrInvalidParam.Msg),
		)
		return nil, common.ErrInvalidParam
	}
	if l.svcCtx.SessionStore == nil {
		logx.L(ctx).Error("session store missing")
		return nil, common.ErrInternalError
	}

	currentHash := auth.HashRefreshToken(req.RefreshToken)
	storeCtx, cancel := context.WithTimeout(ctx, l.svcCtx.Config.AuthSession.StoreTimeout)
	defer cancel()
	session, err := l.svcCtx.SessionStore.GetByRefreshTokenHash(storeCtx, currentHash)
	if err != nil {
		if errors.Is(err, auth.ErrSessionNotFound) {
			return nil, common.ErrUnauthorized
		}
		span.RecordError(err)
		logx.L(ctx).Error("load auth session failed", zap.Error(err))
		return nil, common.ErrInternalError
	}

	now := time.Now()
	if !session.ExpiresAt.After(now.Add(-l.svcCtx.Config.AuthSession.AccessTokenLeeway)) {
		_ = l.svcCtx.SessionStore.Delete(storeCtx, session.SessionID)
		return nil, common.ErrUnauthorized
	}

	userResp, err := l.svcCtx.UserClient.GetUser(ctx, &userrpc.GetUserRequest{UserID: session.UserID})
	if err != nil {
		span.RecordError(err)
		logx.L(ctx).Error("user rpc get user failed", zap.Error(err), zap.Int64("user_id", session.UserID))
		return nil, logicutil.MapRPCError(err)
	}
	if userResp.Code != common.CodeOK {
		return nil, common.NewBizError(userResp.Code, userResp.Message)
	}

	nextRefreshToken, err := auth.NewRefreshToken()
	if err != nil {
		span.RecordError(err)
		logx.L(ctx).Error("refresh token generate failed", zap.Error(err))
		return nil, common.ErrInternalError
	}
	nextRefreshHash := auth.HashRefreshToken(nextRefreshToken)
	nextExpireAt := now.Add(l.svcCtx.Config.AuthSession.RefreshTokenTTL)

	rotated, err := l.svcCtx.SessionStore.Rotate(storeCtx, session.SessionID, currentHash, nextRefreshHash, nextExpireAt, now, userResp.Username, userResp.Role)
	if err != nil {
		if errors.Is(err, auth.ErrSessionConflict) || errors.Is(err, auth.ErrSessionNotFound) {
			return nil, common.ErrUnauthorized
		}
		span.RecordError(err)
		logx.L(ctx).Error("rotate auth session failed", zap.Error(err), zap.String("session_id", session.SessionID))
		return nil, common.ErrInternalError
	}

	data, bizErr := buildTokenResponse(ctx, l.svcCtx, rotated.SessionID, rotated.UserID, rotated.Username, rotated.Role, nextRefreshToken, rotated.ExpiresAt)
	if bizErr != nil {
		return nil, bizErr
	}

	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return data, nil
}
