package user

import (
	"context"
	"strings"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	userrpc "meshcart/gateway/rpc/user"

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
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.user.login", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()

	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		span.SetStatus(codes.Error, common.ErrInvalidParam.Msg)
		return nil, common.ErrInvalidParam
	}

	resp, err := l.svcCtx.UserClient.Login(ctx, &userrpc.LoginRequest{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "user rpc login failed")
		logx.L(ctx).Error("user rpc login failed", zap.Error(err))
		return nil, common.ErrInternalError
	}
	if resp.Code != common.CodeOK {
		span.SetStatus(codes.Error, resp.Message)
		return nil, common.NewBizError(resp.Code, resp.Message)
	}
	span.SetStatus(codes.Ok, "ok")

	return &types.UserLoginData{
		UserID:   resp.UserID,
		Token:    resp.Token,
		Username: resp.Username,
	}, nil
}
