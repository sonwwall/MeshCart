package product

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	"meshcart/gateway/internal/authz"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	productpb "meshcart/kitex_gen/meshcart/product"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type ChangeStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChangeStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangeStatusLogic {
	return &ChangeStatusLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *ChangeStatusLogic) Change(productID int64, status int32, identity *middleware.AuthIdentity) *common.BizError {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.product.change_status", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "product"), attribute.String("biz.action", "change_status"), attribute.Int64("product_id", productID))

	if productID <= 0 {
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "business"), attribute.Int("biz.code", int(common.ErrInvalidParam.Code)), attribute.String("biz.message", common.ErrInvalidParam.Msg))
		return common.ErrInvalidParam
	}
	if identity == nil {
		return common.ErrUnauthorized
	}
	detailResp, err := l.svcCtx.ProductClient.GetProductDetail(ctx, &productpb.GetProductDetailRequest{ProductId: productID})
	if err != nil {
		logx.L(ctx).Error("product rpc detail before status change failed", zap.Error(err))
		return common.ErrInternalError
	}
	if detailResp.Code != common.CodeOK || detailResp.Product == nil {
		return common.NewBizError(detailResp.Code, detailResp.Message)
	}
	role := roleOf(l.svcCtx, identity)
	if !l.svcCtx.AccessControl.Enforce(role, "product", authz.ActionWriteOwn, detailResp.Product.GetCreatorId(), identity.UserID, detailResp.Product.GetStatus()) {
		logx.L(ctx).Warn("product status change forbidden",
			zap.Int64("product_id", productID),
			zap.Int64("user_id", identity.UserID),
			zap.String("role", role),
			zap.Int64("creator_id", detailResp.Product.GetCreatorId()),
			zap.Int32("status", detailResp.Product.GetStatus()),
		)
		if role == authz.RoleAdmin {
			return errOwnProductRequired
		}
		return common.ErrForbidden
	}

	resp, err := l.svcCtx.ProductClient.ChangeProductStatus(ctx, &productpb.ChangeProductStatusRequest{
		ProductId:  productID,
		Status:     status,
		OperatorId: identity.UserID,
	})
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "technical"))
		span.SetStatus(codes.Error, "product rpc change status failed")
		logx.L(ctx).Error("product rpc change status failed", zap.Error(err))
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
