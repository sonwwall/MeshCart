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

type ListByOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListByOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListByOrderLogic {
	return &ListByOrderLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *ListByOrderLogic) ListByOrder(userID, orderID int64) (*types.PaymentListData, *common.BizError) {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.payment.list_by_order", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "payment"), attribute.String("biz.action", "list_by_order"), attribute.Int64("user_id", userID), attribute.Int64("order_id", orderID))

	if userID <= 0 || orderID <= 0 {
		return nil, common.ErrInvalidParam
	}

	resp, err := l.svcCtx.PaymentClient.ListPaymentsByOrder(ctx, &paymentpb.ListPaymentsByOrderRequest{OrderId: orderID, UserId: userID})
	if err != nil {
		span.RecordError(err)
		logx.L(ctx).Error("payment rpc list by order failed", zap.Error(err), zap.Int64("user_id", userID), zap.Int64("order_id", orderID))
		return nil, logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		logx.L(ctx).Warn("payment rpc list by order returned business error",
			zap.Int64("user_id", userID),
			zap.Int64("order_id", orderID),
			zap.Int32("code", resp.Code),
			zap.String("message", resp.Message),
		)
		return nil, common.NewBizError(resp.Code, resp.Message)
	}

	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	logx.L(ctx).Info("list payments by order succeeded",
		zap.Int64("user_id", userID),
		zap.Int64("order_id", orderID),
		zap.Int("payment_count", len(resp.Payments)),
	)
	return toPaymentListData(resp.Payments), nil
}
