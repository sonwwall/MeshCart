package service

import (
	"context"

	"meshcart/app/common"
	orderpb "meshcart/kitex_gen/meshcart/order"
)

func (s *OrderService) ListOrders(ctx context.Context, req *orderpb.ListOrdersRequest) ([]*orderpb.Order, int64, *common.BizError) {
	if req == nil || req.GetUserId() <= 0 {
		return nil, 0, common.ErrInvalidParam
	}

	page := req.GetPage()
	if page <= 0 {
		page = 1
	}
	pageSize := req.GetPageSize()
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := int((page - 1) * pageSize)

	orders, total, err := s.repo.ListByUserID(ctx, req.GetUserId(), offset, int(pageSize))
	if err != nil {
		return nil, 0, mapRepositoryError(err)
	}
	return toRPCOrders(orders), total, nil
}
