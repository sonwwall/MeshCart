package user

import (
	"context"
	"strings"
	"time"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	"meshcart/gateway/internal/auth"
	"meshcart/gateway/internal/logic/logicutil"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	userrpc "meshcart/gateway/rpc/user"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type LoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginLogic) Login(req *types.UserLoginRequest) (*types.UserLoginData, *common.BizError) {
	// 业务层 internal span：用于观察网关内部业务编排耗时。
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.user.login", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "user"), attribute.String("biz.action", "login"))

	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		span.SetAttributes(
			attribute.Bool("biz.success", false),
			attribute.String("biz.type", "business"),
			attribute.Int("biz.code", int(common.ErrInvalidParam.Code)),
			attribute.String("biz.message", common.ErrInvalidParam.Msg),
		)
		return nil, common.ErrInvalidParam
	}

	// 使用同一个 ctx 调下游 RPC，trace 会沿着 ctx 继续传播。
	resp, err := l.svcCtx.UserClient.Login(ctx, &userrpc.LoginRequest{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(
			attribute.Bool("biz.success", false),
			attribute.String("biz.type", "technical"),
		)
		span.SetStatus(codes.Error, "user rpc login failed")
		logx.L(ctx).Error("user rpc login failed", zap.Error(err))
		return nil, logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		logx.L(ctx).Warn("user rpc login returned business error",
			zap.String("username", req.Username),
			zap.Int32("code", resp.Code),
			zap.String("message", resp.Message),
		)
		span.SetAttributes(
			attribute.Bool("biz.success", false),
			attribute.String("biz.type", "business"),
			attribute.Int("biz.code", int(resp.Code)),
			attribute.String("biz.message", resp.Message),
		)
		return nil, common.NewBizError(resp.Code, resp.Message)
	}
	if resp.Username == "" {
		logx.L(ctx).Warn("user rpc login returned empty username",
			zap.String("username", req.Username),
			zap.Int64("user_id", resp.UserID),
		)
	}
	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")

	sessionID := auth.NewSessionID()
	accessToken, accessExpire, err := l.svcCtx.JWT.TokenGenerator(&middleware.AuthIdentity{
		SessionID: sessionID,
		UserID:    resp.UserID,
		Username:  resp.Username,
		Role:      resp.Role,
	})
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(
			attribute.Bool("biz.success", false),
			attribute.String("biz.type", "technical"),
		)
		span.SetStatus(codes.Error, "jwt token generate failed")
		logx.L(ctx).Error("jwt token generate failed", zap.Error(err))
		return nil, common.ErrInternalError
	}

	refreshToken, err := auth.NewRefreshToken()
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(
			attribute.Bool("biz.success", false),
			attribute.String("biz.type", "technical"),
		)
		span.SetStatus(codes.Error, "refresh token generate failed")
		logx.L(ctx).Error("refresh token generate failed", zap.Error(err))
		return nil, common.ErrInternalError
	}

	if l.svcCtx.SessionStore == nil {
		logx.L(ctx).Error("session store missing")
		return nil, common.ErrInternalError
	}

	now := time.Now()
	refreshExpireAt := now.Add(l.svcCtx.Config.AuthSession.RefreshTokenTTL)
	storeCtx, cancel := context.WithTimeout(ctx, l.svcCtx.Config.AuthSession.StoreTimeout)
	defer cancel()
	if err := l.svcCtx.SessionStore.Save(storeCtx, &auth.Session{
		SessionID:        sessionID,
		UserID:           resp.UserID,
		Username:         resp.Username,
		Role:             resp.Role,
		RefreshTokenHash: auth.HashRefreshToken(refreshToken),
		ExpiresAt:        refreshExpireAt,
		CreatedAt:        now,
		UpdatedAt:        now,
	}); err != nil {
		span.RecordError(err)
		span.SetAttributes(
			attribute.Bool("biz.success", false),
			attribute.String("biz.type", "technical"),
		)
		span.SetStatus(codes.Error, "auth session save failed")
		logx.L(ctx).Error("auth session save failed", zap.Error(err))
		return nil, common.ErrInternalError
	}

	return &types.UserLoginData{
		UserID:          resp.UserID,
		Username:        resp.Username,
		Role:            resp.Role,
		SessionID:       sessionID,
		TokenType:       "Bearer",
		AccessToken:     middleware.FormatBearerToken(accessToken),
		AccessExpireAt:  accessExpire.Format(time.RFC3339),
		RefreshToken:    refreshToken,
		RefreshExpireAt: refreshExpireAt.Format(time.RFC3339),
	}, nil
}
