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

type ListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListLogic {
	return &ListLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *ListLogic) List(userID int64, req *types.ListOrdersRequest) (*types.OrderListData, *common.BizError) {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.order.list", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "order"), attribute.String("biz.action", "list"), attribute.Int64("user_id", userID))

	if userID <= 0 || req == nil {
		return nil, common.ErrInvalidParam
	}

	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	resp, err := l.svcCtx.OrderClient.ListOrders(ctx, &orderpb.ListOrdersRequest{
		UserId:   userID,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		span.RecordError(err)
		logx.L(ctx).Error("order rpc list failed", zap.Error(err), zap.Int64("user_id", userID))
		return nil, logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		logx.L(ctx).Warn("order rpc list returned business error",
			zap.Int64("user_id", userID),
			zap.Int32("page", page),
			zap.Int32("page_size", pageSize),
			zap.Int32("code", resp.Code),
			zap.String("message", resp.Message),
		)
		return nil, common.NewBizError(resp.Code, resp.Message)
	}

	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return toOrderListData(resp.Orders, resp.Total), nil
}
