package order

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	"meshcart/gateway/internal/logic/logicutil"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	orderpb "meshcart/kitex_gen/meshcart/order"

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

func (l *GetLogic) Get(userID, orderID int64) (*types.OrderData, *common.BizError) {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.order.get", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "order"), attribute.String("biz.action", "get"), attribute.Int64("user_id", userID), attribute.Int64("order_id", orderID))

	if userID <= 0 || orderID <= 0 {
		return nil, common.ErrInvalidParam
	}

	resp, err := l.svcCtx.OrderClient.GetOrder(ctx, &orderpb.GetOrderRequest{UserId: userID, OrderId: orderID})
	if err != nil {
		span.RecordError(err)
		logx.L(ctx).Error("order rpc get failed", zap.Error(err), zap.Int64("user_id", userID), zap.Int64("order_id", orderID))
		return nil, logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		return nil, common.NewBizError(resp.Code, resp.Message)
	}
	if resp.Order == nil {
		return nil, common.ErrInternalError
	}

	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return toOrderData(resp.Order), nil
}
