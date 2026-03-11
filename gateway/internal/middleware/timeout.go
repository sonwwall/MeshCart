package middleware

import (
	"context"
	"errors"
	"time"

	"meshcart/app/common"
	tracex "meshcart/app/trace"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// RequestTimeout adds a total request deadline for handlers that respect context cancellation.
func RequestTimeout(timeout time.Duration) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if timeout <= 0 {
			c.Next(ctx)
			return
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		c.Next(timeoutCtx)

		if !errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
			return
		}
		if len(c.Response.Body()) > 0 {
			return
		}

		traceID := TraceIDFromRequest(c)
		if traceID == "" {
			traceID = tracex.TraceID(timeoutCtx)
		}

		c.Abort()
		c.JSON(consts.StatusOK, common.Fail(common.ErrServiceBusy, traceID))
	}
}
