package payment

import (
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/route"
)

func RegisterRoutes(api *route.RouterGroup, svcCtx *svc.ServiceContext) {
	paymentGroup := api.Group("/payments")
	paymentGroup.Use(svcCtx.JWT.MiddlewareFunc())
	paymentGroup.POST("", CreatePayment(svcCtx))
	paymentGroup.GET("/:payment_id", GetPayment(svcCtx))
	paymentGroup.POST("/:payment_id/close", ClosePayment(svcCtx))
	paymentGroup.POST("/:payment_id/mock_success", ConfirmMockSuccess(svcCtx))

	orderPaymentGroup := api.Group("/orders")
	orderPaymentGroup.Use(svcCtx.JWT.MiddlewareFunc())
	orderPaymentGroup.GET("/:order_id/payments", ListPaymentsByOrder(svcCtx))
}
