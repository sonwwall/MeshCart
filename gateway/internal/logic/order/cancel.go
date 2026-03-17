package order

import (
	"context"
	"strings"

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

type CancelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCancelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelLogic {
	return &CancelLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *CancelLogic) Cancel(userID, orderID int64, req *types.CancelOrderRequest) (*types.OrderData, *common.BizError) {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.order.cancel", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "order"), attribute.String("biz.action", "cancel"), attribute.Int64("user_id", userID), attribute.Int64("order_id", orderID))

	if userID <= 0 || orderID <= 0 {
		return nil, common.ErrInvalidParam
	}
	if req == nil {
		req = &types.CancelOrderRequest{}
	}

	rpcReq := &orderpb.CancelOrderRequest{
		UserId:  userID,
		OrderId: orderID,
	}
	if reason := strings.TrimSpace(req.CancelReason); reason != "" {
		rpcReq.CancelReason = &reason
	}
	if requestID := strings.TrimSpace(req.RequestID); requestID != "" {
		rpcReq.RequestId = &requestID
	}

	resp, err := l.svcCtx.OrderClient.CancelOrder(ctx, rpcReq)
	if err != nil {
		span.RecordError(err)
		logx.L(ctx).Error("order rpc cancel failed", zap.Error(err), zap.Int64("user_id", userID), zap.Int64("order_id", orderID))
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
