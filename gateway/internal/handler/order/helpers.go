package order

import (
	"strconv"

	"meshcart/app/common"

	"github.com/cloudwego/hertz/pkg/app"
)

func parseOrderID(c *app.RequestContext) (int64, *common.BizError) {
	raw := c.Param("order_id")
	if raw == "" {
		return 0, common.ErrInvalidParam
	}
	orderID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || orderID <= 0 {
		return 0, common.ErrInvalidParam
	}
	return orderID, nil
}
