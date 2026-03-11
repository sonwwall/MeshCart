package product

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	"meshcart/gateway/internal/authz"
	"meshcart/gateway/internal/logic/logicutil"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	productpb "meshcart/kitex_gen/meshcart/product"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type DetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DetailLogic {
	return &DetailLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *DetailLogic) Get(productID int64, identity *middleware.AuthIdentity) (*types.ProductDetailData, *common.BizError) {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.product.detail", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "product"), attribute.String("biz.action", "detail"), attribute.Int64("product_id", productID))

	if productID <= 0 {
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "business"), attribute.Int("biz.code", int(common.ErrInvalidParam.Code)), attribute.String("biz.message", common.ErrInvalidParam.Msg))
		return nil, common.ErrInvalidParam
	}

	resp, err := l.svcCtx.ProductClient.GetProductDetail(ctx, &productpb.GetProductDetailRequest{ProductId: productID})
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "technical"))
		span.SetStatus(codes.Error, "product rpc detail failed")
		logx.L(ctx).Error("product rpc detail failed", zap.Error(err))
		return nil, logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "business"), attribute.Int("biz.code", int(resp.Code)), attribute.String("biz.message", resp.Message))
		return nil, common.NewBizError(resp.Code, resp.Message)
	}

	product := resp.Product
	if product == nil {
		return nil, common.ErrNotFound
	}
	if product.GetStatus() != 2 {
		if identity == nil || !l.svcCtx.AccessControl.Enforce(roleOf(identity), "product", authz.ActionReadPrivate, product.GetCreatorId(), identity.UserID, product.GetStatus()) {
			return nil, common.ErrNotFound
		}
	}

	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return toDetailData(product), nil
}
