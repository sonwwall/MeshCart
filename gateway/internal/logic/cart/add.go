package cart

import (
	"context"
	"strings"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	"meshcart/gateway/internal/logic/logicutil"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	cartpb "meshcart/kitex_gen/meshcart/cart"
	productpb "meshcart/kitex_gen/meshcart/product"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type AddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddLogic {
	return &AddLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *AddLogic) Add(userID int64, req *types.AddCartItemRequest) (*types.CartItemData, *common.BizError) {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.cart.add", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "cart"), attribute.String("biz.action", "add"), attribute.Int64("user_id", userID))

	if userID <= 0 || req == nil || req.ProductID <= 0 || req.SKUID <= 0 || req.Quantity <= 0 {
		return nil, common.ErrInvalidParam
	}

	detailResp, err := l.svcCtx.ProductClient.GetProductDetail(ctx, &productpb.GetProductDetailRequest{ProductId: req.ProductID})
	if err != nil {
		span.RecordError(err)
		logx.L(ctx).Error("product rpc detail before add cart failed", zap.Error(err))
		return nil, logicutil.MapRPCError(err)
	}
	if detailResp.Code != common.CodeOK || detailResp.Product == nil {
		return nil, common.NewBizError(detailResp.Code, detailResp.Message)
	}
	if detailResp.Product.GetStatus() != productStatusOnline {
		return nil, common.ErrNotFound
	}

	sku := findSKU(detailResp.Product, req.SKUID)
	if sku == nil || sku.GetStatus() != skuStatusActive {
		return nil, common.ErrNotFound
	}

	addResp, err := l.svcCtx.CartClient.AddCartItem(ctx, &cartpb.AddCartItemRequest{
		UserId:            userID,
		ProductId:         req.ProductID,
		SkuId:             req.SKUID,
		Quantity:          req.Quantity,
		Checked:           req.Checked,
		TitleSnapshot:     strings.TrimSpace(detailResp.Product.GetTitle()),
		SkuTitleSnapshot:  strings.TrimSpace(sku.GetTitle()),
		SalePriceSnapshot: sku.GetSalePrice(),
		CoverUrlSnapshot:  strings.TrimSpace(sku.GetCoverUrl()),
	})
	if err != nil {
		span.RecordError(err)
		logx.L(ctx).Error("cart rpc add failed", zap.Error(err))
		return nil, logicutil.MapRPCError(err)
	}
	if addResp.Code != common.CodeOK {
		return nil, common.NewBizError(addResp.Code, addResp.Message)
	}
	if addResp.Item == nil {
		return nil, common.ErrInternalError
	}

	item := toCartData([]*cartpb.CartItem{addResp.Item}).Items[0]
	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return &item, nil
}
