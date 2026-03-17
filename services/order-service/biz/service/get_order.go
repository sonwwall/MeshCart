package service

import (
	"context"

	"meshcart/app/common"
	orderpb "meshcart/kitex_gen/meshcart/order"
)

func (s *OrderService) GetOrder(ctx context.Context, req *orderpb.GetOrderRequest) (*orderpb.Order, *common.BizError) {
	if req == nil || req.GetUserId() <= 0 || req.GetOrderId() <= 0 {
		return nil, common.ErrInvalidParam
	}

	order, err := s.repo.GetByOrderID(ctx, req.GetUserId(), req.GetOrderId())
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return toRPCOrder(order), nil
}
