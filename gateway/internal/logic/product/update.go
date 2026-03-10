package product

import (
	"context"
	"strings"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	productpb "meshcart/kitex_gen/meshcart/product"

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

func (l *UpdateLogic) Update(productID int64, req *types.UpdateProductRequest) *common.BizError {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.product.update", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "product"), attribute.String("biz.action", "update"), attribute.Int64("product_id", productID))

	if productID <= 0 || strings.TrimSpace(req.Title) == "" || len(req.SKUs) == 0 {
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "business"), attribute.Int("biz.code", int(common.ErrInvalidParam.Code)), attribute.String("biz.message", common.ErrInvalidParam.Msg))
		return common.ErrInvalidParam
	}

	resp, err := l.svcCtx.ProductClient.UpdateProduct(ctx, &productpb.UpdateProductRequest{
		ProductId:   productID,
		Title:       req.Title,
		SubTitle:    req.SubTitle,
		CategoryId:  req.CategoryID,
		Brand:       req.Brand,
		Description: req.Description,
		Status:      req.Status,
		Skus:        buildSKUInputs(req.SKUs),
	})
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "technical"))
		span.SetStatus(codes.Error, "product rpc update failed")
		logx.L(ctx).Error("product rpc update failed", zap.Error(err))
		return common.ErrInternalError
	}
	if resp.Code != common.CodeOK {
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "business"), attribute.Int("biz.code", int(resp.Code)), attribute.String("biz.message", resp.Message))
		return common.NewBizError(resp.Code, resp.Message)
	}

	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return nil
}
