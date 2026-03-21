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

type CreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateLogic {
	return &CreateLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateLogic) Create(userID int64, req *types.CreateOrderRequest) (*types.OrderData, *common.BizError) {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.order.create", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "order"), attribute.String("biz.action", "create"), attribute.Int64("user_id", userID))

	if userID <= 0 || req == nil || len(req.Items) == 0 {
		return nil, common.ErrInvalidParam
	}
	requestID := strings.TrimSpace(req.RequestID)
	if requestID == "" {
		return nil, common.ErrInvalidParam
	}

	items := make([]*orderpb.OrderItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		if item.ProductID <= 0 || item.SKUID <= 0 || item.Quantity <= 0 {
			return nil, common.ErrInvalidParam
		}
		items = append(items, &orderpb.OrderItemInput{
			ProductId: item.ProductID,
			SkuId:     item.SKUID,
			Quantity:  item.Quantity,
		})
	}

	rpcReq := &orderpb.CreateOrderRequest{
		UserId:    userID,
		Items:     items,
		RequestId: &requestID,
	}

	resp, err := l.svcCtx.OrderClient.CreateOrder(ctx, rpcReq)
	if err != nil {
		span.RecordError(err)
		logx.L(ctx).Error("order rpc create failed", zap.Error(err), zap.Int64("user_id", userID))
		return nil, logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		logx.L(ctx).Warn("order rpc create returned business error",
			zap.Int64("user_id", userID),
			zap.String("request_id", rpcReq.GetRequestId()),
			zap.Int("item_count", len(rpcReq.GetItems())),
			zap.Int32("code", resp.Code),
			zap.String("message", resp.Message),
		)
		return nil, common.NewBizError(resp.Code, resp.Message)
	}
	if resp.Order == nil {
		logx.L(ctx).Error("order rpc create returned nil order",
			zap.Int64("user_id", userID),
			zap.String("request_id", rpcReq.GetRequestId()),
		)
		return nil, common.ErrInternalError
	}

	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return toOrderData(resp.Order), nil
}
