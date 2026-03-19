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

type CloseLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCloseLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseLogic {
	return &CloseLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *CloseLogic) Close(userID, paymentID int64, req *types.ClosePaymentRequest) (*types.PaymentData, *common.BizError) {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.payment.close", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "payment"), attribute.String("biz.action", "close"), attribute.Int64("user_id", userID), attribute.Int64("payment_id", paymentID))

	if userID <= 0 || paymentID <= 0 {
		return nil, common.ErrInvalidParam
	}
	if req == nil {
		req = &types.ClosePaymentRequest{}
	}

	rpcReq := &paymentpb.ClosePaymentRequest{
		PaymentId: paymentID,
		UserId:    userID,
	}
	if requestID := strings.TrimSpace(req.RequestID); requestID != "" {
		rpcReq.RequestId = &requestID
	}
	if reason := strings.TrimSpace(req.Reason); reason != "" {
		rpcReq.Reason = &reason
	}

	resp, err := l.svcCtx.PaymentClient.ClosePayment(ctx, rpcReq)
	if err != nil {
		span.RecordError(err)
		logx.L(ctx).Error("payment rpc close failed", zap.Error(err), zap.Int64("user_id", userID), zap.Int64("payment_id", paymentID))
		return nil, logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		logx.L(ctx).Warn("payment rpc close returned business error",
			zap.Int64("user_id", userID),
			zap.Int64("payment_id", paymentID),
			zap.Int32("code", resp.Code),
			zap.String("message", resp.Message),
		)
		return nil, common.NewBizError(resp.Code, resp.Message)
	}
	if resp.Payment == nil {
		logx.L(ctx).Error("payment rpc close returned nil payment",
			zap.Int64("user_id", userID),
			zap.Int64("payment_id", paymentID),
		)
		return nil, common.ErrInternalError
	}

	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return toPaymentData(resp.Payment), nil
}
