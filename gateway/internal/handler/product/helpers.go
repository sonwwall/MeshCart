package product

import (
	"strconv"

	"meshcart/app/common"

	"github.com/cloudwego/hertz/pkg/app"
)

func parseProductID(c *app.RequestContext) (int64, *common.BizError) {
	productID, err := strconv.ParseInt(c.Param("product_id"), 10, 64)
	if err != nil || productID <= 0 {
		return 0, common.ErrInvalidParam
	}
	return productID, nil
}
