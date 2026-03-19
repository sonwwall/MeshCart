package service

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	"meshcart/kitex_gen/meshcart/inventory"
	orderpb "meshcart/kitex_gen/meshcart/order"
	"meshcart/services/order-service/biz/repository"

	"go.uber.org/zap"
)

func (s *OrderService) CloseExpiredOrders(ctx context.Context, req *orderpb.CloseExpiredOrdersRequest) ([]int64, *common.BizError) {
	limit := defaultCloseLimit
	if req != nil && req.GetLimit() > 0 {
		limit = int(req.GetLimit())
	}
	logx.L(ctx).Info("close expired orders start", zap.Int("limit", limit))

	orders, err := s.repo.ListExpiredOrders(ctx, s.now(), limit)
	if err != nil {
		bizErr := mapRepositoryError(err)
		logx.L(ctx).Error("close expired orders list failed",
			zap.Error(err),
			zap.Int("limit", limit),
			zap.Int32("mapped_code", bizErr.Code),
			zap.String("mapped_message", bizErr.Msg),
		)
		return nil, bizErr
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
			logx.L(ctx).Warn("release inventory for expired order failed",
				zap.Error(releaseErr),
				zap.Int64("order_id", order.OrderID),
				zap.String("biz_id", s.reserveBizID(order.OrderID)),
			)
			continue
		}
		if releaseResp.Code != 0 {
			logx.L(ctx).Warn("release inventory for expired order returned biz error",
				zap.Int32("code", releaseResp.Code),
				zap.String("message", releaseResp.Message),
				zap.Int64("order_id", order.OrderID),
			)
			continue
		}

		if _, updateErr := s.repo.TransitionStatus(ctx, repository.OrderTransition{
			OrderID:      order.OrderID,
			FromStatuses: []int32{OrderStatusPending, OrderStatusReserved},
			ToStatus:     OrderStatusClosed,
			CancelReason: "order_expired",
			ActionType:   "close_expired",
			Reason:       "order_expired",
		}); updateErr != nil {
			logx.L(ctx).Warn("close expired order update status failed",
				zap.Error(updateErr),
				zap.Int64("order_id", order.OrderID),
			)
			continue
		}
		logx.L(ctx).Info("close expired order completed",
			zap.Int64("order_id", order.OrderID),
			zap.Int64("user_id", order.UserID),
		)
		closedOrderIDs = append(closedOrderIDs, order.OrderID)
	}
	logx.L(ctx).Info("close expired orders completed",
		zap.Int("limit", limit),
		zap.Int("closed_count", len(closedOrderIDs)),
		zap.Int64s("closed_order_ids", closedOrderIDs),
	)
	return closedOrderIDs, nil
}
