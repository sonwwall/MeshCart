package payment

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	"meshcart/gateway/internal/logic/logicutil"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"

	paymentpb "meshcart/kitex_gen/meshcart/payment"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type GetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLogic {
	return &GetLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *GetLogic) Get(userID, paymentID int64) (*types.PaymentData, *common.BizError) {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.payment.get", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "payment"), attribute.String("biz.action", "get"), attribute.Int64("user_id", userID), attribute.Int64("payment_id", paymentID))

	if userID <= 0 || paymentID <= 0 {
		return nil, common.ErrInvalidParam
	}

	resp, err := l.svcCtx.PaymentClient.GetPayment(ctx, &paymentpb.GetPaymentRequest{PaymentId: paymentID, UserId: userID})
	if err != nil {
		span.RecordError(err)
		logx.L(ctx).Error("payment rpc get failed", zap.Error(err), zap.Int64("user_id", userID), zap.Int64("payment_id", paymentID))
		return nil, logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		return nil, common.NewBizError(resp.Code, resp.Message)
	}
	if resp.Payment == nil {
		return nil, common.ErrInternalError
	}

	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return toPaymentData(resp.Payment), nil
}
