package payment

import (
	"context"
	"strings"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	"meshcart/gateway/internal/logic/logicutil"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	paymentpb "meshcart/kitex_gen/meshcart/payment"

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

func (l *CreateLogic) Create(userID int64, req *types.CreatePaymentRequest) (*types.PaymentData, *common.BizError) {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.payment.create", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "payment"), attribute.String("biz.action", "create"), attribute.Int64("user_id", userID))

	if userID <= 0 || req == nil || req.OrderID <= 0 {
		return nil, common.ErrInvalidParam
	}

	rpcReq := &paymentpb.CreatePaymentRequest{
		OrderId:       req.OrderID,
		UserId:        userID,
		PaymentMethod: strings.TrimSpace(req.PaymentMethod),
	}
	if requestID := strings.TrimSpace(req.RequestID); requestID != "" {
		rpcReq.RequestId = &requestID
	}

	resp, err := l.svcCtx.PaymentClient.CreatePayment(ctx, rpcReq)
	if err != nil {
		span.RecordError(err)
		logx.L(ctx).Error("payment rpc create failed", zap.Error(err), zap.Int64("user_id", userID), zap.Int64("order_id", req.OrderID))
		return nil, logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		logx.L(ctx).Warn("payment rpc create returned business error",
			zap.Int64("user_id", userID),
			zap.Int64("order_id", req.OrderID),
			zap.String("payment_method", rpcReq.GetPaymentMethod()),
			zap.Int32("code", resp.Code),
			zap.String("message", resp.Message),
		)
		return nil, common.NewBizError(resp.Code, resp.Message)
	}
	if resp.Payment == nil {
		logx.L(ctx).Error("payment rpc create returned nil payment",
			zap.Int64("user_id", userID),
			zap.Int64("order_id", req.OrderID),
		)
		return nil, common.ErrInternalError
	}

	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return toPaymentData(resp.Payment), nil
}
