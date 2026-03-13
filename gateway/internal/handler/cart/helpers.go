package cart

import (
	"strconv"

	"meshcart/app/common"

	"github.com/cloudwego/hertz/pkg/app"
)

func parseItemID(c *app.RequestContext) (int64, *common.BizError) {
	itemID, err := strconv.ParseInt(c.Param("item_id"), 10, 64)
	if err != nil || itemID <= 0 {
		return 0, common.ErrInvalidParam
	}
	return itemID, nil
}
