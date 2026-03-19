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

type MockSuccessLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMockSuccessLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MockSuccessLogic {
	return &MockSuccessLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *MockSuccessLogic) Confirm(userID, paymentID int64, req *types.ConfirmMockPaymentRequest) (*types.PaymentData, *common.BizError) {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.payment.mock_success", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "payment"), attribute.String("biz.action", "mock_success"), attribute.Int64("user_id", userID), attribute.Int64("payment_id", paymentID))

	if userID <= 0 || paymentID <= 0 {
		return nil, common.ErrInvalidParam
	}
	if req == nil {
		req = &types.ConfirmMockPaymentRequest{}
	}

	rpcReq := &paymentpb.ConfirmPaymentSuccessRequest{
		PaymentId:     paymentID,
		PaymentMethod: "mock",
	}
	if requestID := strings.TrimSpace(req.RequestID); requestID != "" {
		rpcReq.RequestId = &requestID
	}
	if tradeNo := strings.TrimSpace(req.PaymentTradeNo); tradeNo != "" {
		rpcReq.PaymentTradeNo = &tradeNo
	}
	if req.PaidAt > 0 {
		rpcReq.PaidAt = &req.PaidAt
	}

	resp, err := l.svcCtx.PaymentClient.ConfirmPaymentSuccess(ctx, rpcReq)
	if err != nil {
		span.RecordError(err)
		logx.L(ctx).Error("payment rpc confirm success failed", zap.Error(err), zap.Int64("user_id", userID), zap.Int64("payment_id", paymentID))
		return nil, logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		return nil, common.NewBizError(resp.Code, resp.Message)
	}
	if resp.Payment == nil {
		return nil, common.ErrInternalError
	}

	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return toPaymentData(resp.Payment), nil
}
