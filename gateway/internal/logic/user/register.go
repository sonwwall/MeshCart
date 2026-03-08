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

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type RegisterLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RegisterLogic) Register(req *types.UserRegisterRequest) *common.BizError {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.user.register", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "user"), attribute.String("biz.action", "register"))

	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		span.SetAttributes(
			attribute.Bool("biz.success", false),
			attribute.String("biz.type", "business"),
			attribute.Int("biz.code", int(common.ErrInvalidParam.Code)),
			attribute.String("biz.message", common.ErrInvalidParam.Msg),
		)
		return common.ErrInvalidParam
	}

	resp, err := l.svcCtx.UserClient.Register(ctx, &userrpc.RegisterRequest{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(
			attribute.Bool("biz.success", false),
			attribute.String("biz.type", "technical"),
		)
		span.SetStatus(codes.Error, "user rpc register failed")
		logx.L(ctx).Error("user rpc register failed", zap.Error(err))
		return common.ErrInternalError
	}
	if resp.Code != common.CodeOK {
		span.SetAttributes(
			attribute.Bool("biz.success", false),
			attribute.String("biz.type", "business"),
			attribute.Int("biz.code", int(resp.Code)),
			attribute.String("biz.message", resp.Message),
		)
		return common.NewBizError(resp.Code, resp.Message)
	}

	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return nil
}
