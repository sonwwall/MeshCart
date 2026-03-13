package cart

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	"meshcart/gateway/internal/logic/logicutil"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	cartpb "meshcart/kitex_gen/meshcart/cart"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type UpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateLogic {
	return &UpdateLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateLogic) Update(userID, itemID int64, req *types.UpdateCartItemRequest) (*types.CartItemData, *common.BizError) {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.cart.update", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "cart"), attribute.String("biz.action", "update"), attribute.Int64("user_id", userID), attribute.Int64("item_id", itemID))

	if userID <= 0 || itemID <= 0 || req == nil || req.Quantity <= 0 {
		return nil, common.ErrInvalidParam
	}

	resp, err := l.svcCtx.CartClient.UpdateCartItem(ctx, &cartpb.UpdateCartItemRequest{
		UserId:   userID,
		ItemId:   itemID,
		Quantity: req.Quantity,
		Checked:  req.Checked,
	})
	if err != nil {
		span.RecordError(err)
		logx.L(ctx).Error("cart rpc update failed", zap.Error(err))
		return nil, logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		return nil, common.NewBizError(resp.Code, resp.Message)
	}
	if resp.Item == nil {
		return nil, common.ErrInternalError
	}

	item := toCartData([]*cartpb.CartItem{resp.Item}).Items[0]
	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return &item, nil
}
