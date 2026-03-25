package service

import (
	"context"
	"sort"
	"time"

	"meshcart/app/common"
	logx "meshcart/app/log"
	metricsx "meshcart/app/metrics"
	"meshcart/kitex_gen/meshcart/inventory"
	orderpb "meshcart/kitex_gen/meshcart/order"
	"meshcart/services/order-service/biz/errno"
	dalmodel "meshcart/services/order-service/dal/model"

	"go.uber.org/zap"
)

func (s *OrderService) CreateOrder(ctx context.Context, req *orderpb.CreateOrderRequest) (*orderpb.Order, *common.BizError) {
	startedAt := s.now()
	if req == nil || req.GetUserId() <= 0 || len(req.GetItems()) == 0 {
		userID := int64(0)
		itemCount := 0
		if req != nil {
			userID = req.GetUserId()
			itemCount = len(req.GetItems())
		}
		logx.L(ctx).Warn("create order rejected by invalid request",
			zap.Int64("user_id", userID),
			zap.Int("item_count", itemCount),
		)
		metricsx.ObserveBizError("order-service", "order", "create", "request", common.CodeInvalidParam)
		return nil, common.ErrInvalidParam
	}

	requestID, bizErr := requireRequestID(req.GetRequestId())
	if bizErr != nil {
		logx.L(ctx).Warn("create order rejected by missing request_id",
			zap.Int64("user_id", req.GetUserId()),
			zap.Int("item_count", len(req.GetItems())),
		)
		metricsx.ObserveBizError("order-service", "order", "create", "request", bizErr.Code)
		return nil, bizErr
	}
	logx.L(ctx).Info("create order start",
		zap.Int64("user_id", req.GetUserId()),
		zap.Int("item_count", len(req.GetItems())),
		zap.String("request_id", requestID),
	)
	if requestID != "" {
		existing, bizErr := s.findActionRecord(ctx, actionTypeCreate, requestID)
		if bizErr != nil {
			metricsx.ObserveBizError("order-service", "order", "create", "idempotency_lookup", bizErr.Code)
			return nil, bizErr
		}
		if existing != nil {
			switch existing.Status {
			case "succeeded":
				logx.L(ctx).Info("create order hit succeeded action record",
					zap.String("request_id", requestID),
					zap.Int64("order_id", existing.OrderID),
					zap.Int64("user_id", req.GetUserId()),
				)
				return s.loadOrderByActionRecord(ctx, existing)
			case actionStatusPending:
				logx.L(ctx).Warn("create order blocked by pending action record",
					zap.String("request_id", requestID),
					zap.Int64("user_id", req.GetUserId()),
				)
				metricsx.ObserveBizError("order-service", "order", "create", "idempotency_lookup", errno.CodeOrderIdempotencyBusy)
				return nil, errno.ErrOrderIdempotencyBusy
			default:
				logx.L(ctx).Warn("create order rejected by failed action record",
					zap.String("request_id", requestID),
					zap.Int64("user_id", req.GetUserId()),
					zap.String("status", existing.Status),
				)
				metricsx.ObserveBizError("order-service", "order", "create", "idempotency_lookup", errno.CodeOrderStateConflict)
				return nil, errno.ErrOrderStateConflict
			}
		}
		record, bizErr := s.createPendingActionRecord(ctx, actionTypeCreate, requestID, 0, req.GetUserId())
		if bizErr != nil {
			metricsx.ObserveBizError("order-service", "order", "create", "idempotency_create", bizErr.Code)
			return nil, bizErr
		}
		if record != nil && record.Status != actionStatusPending {
			if record.Status == "succeeded" {
				logx.L(ctx).Info("create order reused succeeded action record after create attempt",
					zap.String("request_id", requestID),
					zap.Int64("order_id", record.OrderID),
					zap.Int64("user_id", req.GetUserId()),
				)
				return s.loadOrderByActionRecord(ctx, record)
			}
			logx.L(ctx).Warn("create order blocked by non-pending action record after create attempt",
				zap.String("request_id", requestID),
				zap.Int64("user_id", req.GetUserId()),
				zap.String("status", record.Status),
			)
			metricsx.ObserveBizError("order-service", "order", "create", "idempotency_create", errno.CodeOrderIdempotencyBusy)
			return nil, errno.ErrOrderIdempotencyBusy
		}
	}

	orderID := s.node.Generate().Int64()
	validationStartedAt := s.now()
	validatedItems, reserveItems, totalAmount, bizErr := s.validateAndBuildSnapshots(ctx, req.GetItems())
	validationDuration := s.now().Sub(validationStartedAt)
	if bizErr != nil {
		metricsx.ObserveBizError("order-service", "order", "create", "validation", bizErr.Code)
		logx.L(ctx).Warn("create order validation failed",
			zap.Int64("order_id", orderID),
			zap.Int64("user_id", req.GetUserId()),
			zap.Int32("code", bizErr.Code),
			zap.String("message", bizErr.Msg),
			zap.Duration("validation_duration", validationDuration),
		)
		s.markActionFailed(ctx, actionTypeCreate, requestID, bizErr)
		return nil, bizErr
	}
	sort.SliceStable(reserveItems, func(i, j int) bool { return reserveItems[i].GetSkuId() < reserveItems[j].GetSkuId() })

	bizID := s.reserveBizID(orderID)
	reserveStartedAt := s.now()
	reserveResp, err := s.inventoryClient.ReserveSkuStocks(ctx, &inventory.ReserveSkuStocksRequest{
		BizType: orderReserveBizType,
		BizId:   bizID,
		Items:   reserveItems,
	})
	reserveDuration := s.now().Sub(reserveStartedAt)
	if err != nil {
		metricsx.ObserveBizError("order-service", "order", "create", "reserve_rpc", common.CodeInternalError)
		logx.L(ctx).Error("reserve inventory failed",
			zap.Error(err),
			zap.Int64("order_id", orderID),
			zap.Int64("user_id", req.GetUserId()),
			zap.String("biz_id", bizID),
			zap.Duration("validation_duration", validationDuration),
			zap.Duration("reserve_duration", reserveDuration),
		)
		s.markActionFailed(ctx, actionTypeCreate, requestID, common.ErrServiceUnavailable)
		return nil, common.ErrServiceUnavailable
	}
	if reserveResp.Code != 0 {
		bizErr = mapInventoryRPCError(reserveResp.Code)
		metricsx.ObserveBizError("order-service", "order", "create", "reserve_biz", bizErr.Code)
		logx.L(ctx).Warn("reserve inventory returned business error",
			zap.Int64("order_id", orderID),
			zap.Int64("user_id", req.GetUserId()),
			zap.String("biz_id", bizID),
			zap.Int32("inventory_rpc_code", reserveResp.Code),
			zap.String("inventory_rpc_message", reserveResp.Message),
			zap.Int32("mapped_code", bizErr.Code),
			zap.String("mapped_message", bizErr.Msg),
			zap.Duration("validation_duration", validationDuration),
			zap.Duration("reserve_duration", reserveDuration),
		)
		s.markActionFailed(ctx, actionTypeCreate, requestID, bizErr)
		return nil, bizErr
	}

	items := make([]*dalmodel.OrderItem, 0, len(validatedItems))
	for _, item := range validatedItems {
		items = append(items, &dalmodel.OrderItem{
			ID:                   s.node.Generate().Int64(),
			OrderID:              orderID,
			ProductID:            item.productID,
			SKUID:                item.skuID,
			ProductTitleSnapshot: item.productTitleSnapshot,
			SKUTitleSnapshot:     item.skuTitleSnapshot,
			SalePriceSnapshot:    item.salePriceSnapshot,
			Quantity:             item.quantity,
			SubtotalAmount:       item.subtotalAmount,
		})
	}

	orderModel := &dalmodel.Order{
		OrderID:      orderID,
		UserID:       req.GetUserId(),
		Status:       OrderStatusReserved,
		TotalAmount:  totalAmount,
		PayAmount:    totalAmount,
		ExpireAt:     s.now().Add(30 * time.Minute),
		CancelReason: "",
	}

	persistStartedAt := s.now()
	order, err := s.repo.CreateWithItems(ctx, orderModel, items)
	persistDuration := s.now().Sub(persistStartedAt)
	if err != nil {
		releaseResp, releaseErr := s.inventoryClient.ReleaseReservedSkuStocks(ctx, &inventory.ReleaseReservedSkuStocksRequest{
			BizType: orderReserveBizType,
			BizId:   bizID,
			Items:   reserveItems,
		})
		if releaseErr != nil {
			logx.L(ctx).Error("release inventory after create order failure failed", zap.Error(releaseErr), zap.Int64("order_id", orderID))
		} else if releaseResp.Code != 0 {
			logx.L(ctx).Error("release inventory after create order failure returned biz error", zap.Int32("code", releaseResp.Code), zap.String("message", releaseResp.Message), zap.Int64("order_id", orderID))
		}
		bizErr = mapRepositoryError(err)
		metricsx.ObserveBizError("order-service", "order", "create", "persist", bizErr.Code)
		logx.L(ctx).Error("create order persist failed",
			zap.Error(err),
			zap.Int64("order_id", orderID),
			zap.Int64("user_id", req.GetUserId()),
			zap.Int32("mapped_code", bizErr.Code),
			zap.String("mapped_message", bizErr.Msg),
			zap.Duration("validation_duration", validationDuration),
			zap.Duration("reserve_duration", reserveDuration),
			zap.Duration("persist_duration", persistDuration),
		)
		s.markActionFailed(ctx, actionTypeCreate, requestID, bizErr)
		return nil, bizErr
	}
	logx.L(ctx).Info("create order completed",
		zap.Int64("order_id", order.OrderID),
		zap.Int64("user_id", order.UserID),
		zap.Int32("status", order.Status),
		zap.Int64("pay_amount", order.PayAmount),
		zap.Time("expire_at", order.ExpireAt),
		zap.Duration("validation_duration", validationDuration),
		zap.Duration("reserve_duration", reserveDuration),
		zap.Duration("persist_duration", persistDuration),
		zap.Duration("total_duration", s.now().Sub(startedAt)),
	)
	s.markActionSucceeded(ctx, actionTypeCreate, requestID, order.OrderID)
	return toRPCOrder(order), nil
}
