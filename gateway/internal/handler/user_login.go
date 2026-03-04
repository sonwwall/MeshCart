package handler

import (
	"context"

	"meshcart/app/common"
	"meshcart/gateway/internal/logic"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"

	"github.com/cloudwego/hertz/pkg/app"
)

func UserLogin(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		traceID := middleware.TraceIDFromRequest(c)

		var req types.UserLoginRequest
		if err := c.BindAndValidate(&req); err != nil {
			c.JSON(200, common.Fail(common.ErrInvalidParam, traceID))
			return
		}

		loginLogic := logic.NewUserLoginLogic(ctx, svcCtx)
		data, bizErr := loginLogic.Login(&req)
		if bizErr != nil {
			c.JSON(200, common.Fail(bizErr, traceID))
			return
		}

		c.JSON(200, common.Success(data, traceID))
	}
}
