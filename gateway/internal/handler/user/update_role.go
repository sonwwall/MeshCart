package user

import (
	"context"
	"strconv"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	userlogic "meshcart/gateway/internal/logic/user"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"
)

func UpdateUserRole(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		traceID := middleware.TraceIDFromRequest(c)
		if traceID == "" {
			traceID = tracex.TraceID(ctx)
		}
		ctx = logx.WithTraceID(ctx, traceID)

		userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
		if err != nil || userID <= 0 {
			c.JSON(200, common.Fail(common.ErrInvalidParam, traceID))
			return
		}

		var req types.UpdateUserRoleRequest
		if err := c.BindAndValidate(&req); err != nil {
			logx.L(ctx).Warn("update user role request bind failed", zap.Error(err))
			c.JSON(200, common.Fail(common.ErrInvalidParam, traceID))
			return
		}

		identity, ok := middleware.IdentityFromRequest(ctx, c)
		if !ok {
			c.JSON(200, common.Fail(common.ErrUnauthorized, traceID))
			return
		}

		logic := userlogic.NewUpdateUserRoleLogic(ctx, svcCtx)
		bizErr := logic.Update(userID, req.Role, identity)
		if bizErr != nil {
			logx.L(ctx).Warn("update user role failed",
				zap.Int64("target_user_id", userID),
				zap.String("role", req.Role),
				zap.Int32("code", bizErr.Code),
				zap.String("message", bizErr.Msg),
			)
			c.JSON(200, common.Fail(bizErr, traceID))
			return
		}

		c.JSON(200, common.Success(nil, traceID))
	}
}
