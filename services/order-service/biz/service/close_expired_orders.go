package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	"meshcart/kitex_gen/meshcart/inventory"
	orderpb "meshcart/kitex_gen/meshcart/order"

	"go.uber.org/zap"
)

func (s *OrderService) CloseExpiredOrders(ctx context.Context, req *orderpb.CloseExpiredOrdersRequest) ([]int64, *common.BizError) {
	limit := defaultCloseLimit
	if req != nil && req.GetLimit() > 0 {
		limit = int(req.GetLimit())
	}

	orders, err := s.repo.ListExpiredOrders(ctx, s.now(), limit)
	if err != nil {
		return nil, mapRepositoryError(err)
	}

	closedOrderIDs := make([]int64, 0, len(orders))
	for _, order := range orders {
		if order == nil {
			continue
		}
		releaseResp, releaseErr := s.inventoryClient.ReleaseReservedSkuStocks(ctx, &inventory.ReleaseReservedSkuStocksRequest{
			BizType: orderReserveBizType,
			BizId:   s.reserveBizID(order.OrderID),
			Items:   buildReleaseItems(order),
		})
		if releaseErr != nil {
			logx.L(ctx).Warn("release inventory for expired order failed", zap.Error(releaseErr), zap.Int64("order_id", order.OrderID))
			continue
		}
		if releaseResp.Code != 0 {
			logx.L(ctx).Warn("release inventory for expired order returned biz error", zap.Int32("code", releaseResp.Code), zap.String("message", releaseResp.Message), zap.Int64("order_id", order.OrderID))
			continue
		}

		if _, updateErr := s.repo.UpdateStatus(ctx, order.OrderID, []int32{OrderStatusPending, OrderStatusReserved}, OrderStatusClosed, "order_expired"); updateErr != nil {
			logx.L(ctx).Warn("close expired order update status failed", zap.Error(updateErr), zap.Int64("order_id", order.OrderID))
			continue
		}
		closedOrderIDs = append(closedOrderIDs, order.OrderID)
	}
	return closedOrderIDs, nil
}
