package dtm

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/dtm-labs/dtm/client/dtmcli"
	"github.com/dtm-labs/dtm/client/workflow"
)

func WorkflowCallback() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		name := string(c.Query("op"))
		gid := string(c.Query("gid"))
		_, err := workflow.ExecuteCtx(ctx, name, gid, c.Request.Body())
		code, res := dtmcli.Result2HttpJSON(err)
		c.JSON(code, res)
	}
}
