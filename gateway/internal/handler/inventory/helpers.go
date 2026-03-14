package inventory

import (
	"strconv"

	"meshcart/app/common"

	"github.com/cloudwego/hertz/pkg/app"
)

func parseSKUID(c *app.RequestContext) (int64, *common.BizError) {
	skuID, err := strconv.ParseInt(c.Param("sku_id"), 10, 64)
	if err != nil || skuID <= 0 {
		return 0, common.ErrInvalidParam
	}
	return skuID, nil
}
