package cart

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	"meshcart/gateway/internal/logic/logicutil"
	"meshcart/gateway/internal/svc"
	cartpb "meshcart/kitex_gen/meshcart/cart"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type RemoveLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRemoveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RemoveLogic {
	return &RemoveLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *RemoveLogic) Remove(userID, itemID int64) *common.BizError {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.cart.remove", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "cart"), attribute.String("biz.action", "remove"), attribute.Int64("user_id", userID), attribute.Int64("item_id", itemID))

	if userID <= 0 || itemID <= 0 {
		return common.ErrInvalidParam
	}

	resp, err := l.svcCtx.CartClient.RemoveCartItem(ctx, &cartpb.RemoveCartItemRequest{UserId: userID, ItemId: itemID})
	if err != nil {
		span.RecordError(err)
		logx.L(ctx).Error("cart rpc remove failed", zap.Error(err))
		return logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		logx.L(ctx).Warn("cart rpc remove returned business error",
			zap.Int64("user_id", userID),
			zap.Int64("item_id", itemID),
			zap.Int32("code", resp.Code),
			zap.String("message", resp.Message),
		)
		return common.NewBizError(resp.Code, resp.Message)
	}
	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return nil
}
