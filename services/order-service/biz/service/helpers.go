package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"meshcart/app/common"
	logx "meshcart/app/log"
	"meshcart/kitex_gen/meshcart/inventory"
	orderpb "meshcart/kitex_gen/meshcart/order"
	productpb "meshcart/kitex_gen/meshcart/product"
	"meshcart/services/order-service/biz/errno"
	"meshcart/services/order-service/biz/repository"
	dalmodel "meshcart/services/order-service/dal/model"

	"go.uber.org/zap"
)

const (
	productStatusOnline int32 = 2
	skuStatusActive     int32 = 1

	orderReserveBizType = "order"
	defaultCloseLimit   = 100
	actionStatusPending = "pending"
	actionTypeCreate    = "create"
	actionTypeCancel    = "cancel"
	actionTypePay       = "pay_confirm"
)

type validatedOrderItem struct {
	productID            int64
	skuID                int64
	productTitleSnapshot string
	skuTitleSnapshot     string
	salePriceSnapshot    int64
	quantity             int32
	subtotalAmount       int64
}

func (s *OrderService) now() time.Time {
	if s.nowFunc != nil {
		return s.nowFunc()
	}
	return time.Now()
}

func (s *OrderService) reserveBizID(orderID int64) string {
	return fmt.Sprintf("%d", orderID)
}

func (s *OrderService) validateAndBuildSnapshots(ctx context.Context, reqItems []*orderpb.OrderItemInput) ([]validatedOrderItem, []*inventory.StockReservationItem, int64, *common.BizError) {
	if s.productClient == nil || s.inventoryClient == nil {
		logx.L(ctx).Error("order service downstream client is not initialized")
		return nil, nil, 0, common.ErrInternalError
	}

	skuIDs := make([]int64, 0, len(reqItems))
	requestedProductBySKU := make(map[int64]int64, len(reqItems))
	reservationQuantity := make(map[int64]int64, len(reqItems))
	for _, item := range reqItems {
		if item == nil || item.GetProductId() <= 0 || item.GetSkuId() <= 0 || item.GetQuantity() <= 0 {
			return nil, nil, 0, common.ErrInvalidParam
		}
		if existingProductID, ok := requestedProductBySKU[item.GetSkuId()]; ok && existingProductID != item.GetProductId() {
			return nil, nil, 0, common.ErrInvalidParam
		}
		if _, ok := requestedProductBySKU[item.GetSkuId()]; !ok {
			skuIDs = append(skuIDs, item.GetSkuId())
			requestedProductBySKU[item.GetSkuId()] = item.GetProductId()
		}
		reservationQuantity[item.GetSkuId()] += int64(item.GetQuantity())
	}

	skuResp, err := s.productClient.BatchGetSKU(ctx, &productpb.BatchGetSkuRequest{SkuIds: skuIDs})
	if err != nil {
		logx.L(ctx).Error("batch get sku failed", zap.Error(err), zap.Int("sku_count", len(skuIDs)))
		return nil, nil, 0, common.ErrServiceUnavailable
	}
	if skuResp.Code != 0 {
		return nil, nil, 0, mapProductRPCError(skuResp.Code)
	}

	skuMap := make(map[int64]*productpb.ProductSku, len(skuResp.Skus))
	productIDs := make([]int64, 0, len(skuResp.Skus))
	productIDSeen := make(map[int64]struct{}, len(skuResp.Skus))
	for _, sku := range skuResp.Skus {
		if sku == nil {
			return nil, nil, 0, common.ErrServiceUnavailable
		}
		expectedProductID, ok := requestedProductBySKU[sku.GetId()]
		if !ok || sku.GetSpuId() != expectedProductID {
			return nil, nil, 0, errno.ErrOrderSKUUnavailable
		}
		if sku.GetStatus() != skuStatusActive {
			return nil, nil, 0, errno.ErrOrderSKUUnavailable
		}
		skuMap[sku.GetId()] = sku
		if _, ok := productIDSeen[sku.GetSpuId()]; !ok {
			productIDs = append(productIDs, sku.GetSpuId())
			productIDSeen[sku.GetSpuId()] = struct{}{}
		}
	}
	if len(skuMap) != len(requestedProductBySKU) {
		return nil, nil, 0, errno.ErrOrderSKUUnavailable
	}

	productMap := make(map[int64]*productpb.Product, len(productIDs))
	for _, productID := range productIDs {
		resp, rpcErr := s.productClient.GetProductDetail(ctx, &productpb.GetProductDetailRequest{ProductId: productID})
		if rpcErr != nil {
			logx.L(ctx).Error("get product detail failed", zap.Error(rpcErr), zap.Int64("product_id", productID))
			return nil, nil, 0, common.ErrServiceUnavailable
		}
		if resp.Code != 0 {
			return nil, nil, 0, mapProductRPCError(resp.Code)
		}
		if resp.Product == nil || resp.Product.GetStatus() != productStatusOnline {
			return nil, nil, 0, errno.ErrOrderProductUnavailable
		}
		productMap[productID] = resp.Product
	}

	items := make([]validatedOrderItem, 0, len(reqItems))
	totalAmount := int64(0)
	for _, reqItem := range reqItems {
		sku := skuMap[reqItem.GetSkuId()]
		product := productMap[reqItem.GetProductId()]
		if sku == nil || product == nil {
			return nil, nil, 0, errno.ErrOrderProductUnavailable
		}
		subtotal := sku.GetSalePrice() * int64(reqItem.GetQuantity())
		totalAmount += subtotal
		items = append(items, validatedOrderItem{
			productID:            reqItem.GetProductId(),
			skuID:                reqItem.GetSkuId(),
			productTitleSnapshot: strings.TrimSpace(product.GetTitle()),
			skuTitleSnapshot:     strings.TrimSpace(sku.GetTitle()),
			salePriceSnapshot:    sku.GetSalePrice(),
			quantity:             reqItem.GetQuantity(),
			subtotalAmount:       subtotal,
		})
	}

	reserveItems := make([]*inventory.StockReservationItem, 0, len(reservationQuantity))
	for skuID, quantity := range reservationQuantity {
		reserveItems = append(reserveItems, &inventory.StockReservationItem{SkuId: skuID, Quantity: quantity})
	}
	return items, reserveItems, totalAmount, nil
}

func buildReleaseItems(order *dalmodel.Order) []*inventory.StockReservationItem {
	quantities := make(map[int64]int64, len(order.Items))
	for _, item := range order.Items {
		quantities[item.SKUID] += int64(item.Quantity)
	}

	items := make([]*inventory.StockReservationItem, 0, len(quantities))
	for skuID, quantity := range quantities {
		items = append(items, &inventory.StockReservationItem{SkuId: skuID, Quantity: quantity})
	}
	return items
}

func (s *OrderService) findActionRecord(ctx context.Context, actionType, actionKey string) (*dalmodel.OrderActionRecord, *common.BizError) {
	record, err := s.repo.GetActionRecord(ctx, actionType, actionKey)
	if err == nil {
		return record, nil
	}
	if err == repository.ErrActionRecordNotFound {
		return nil, nil
	}
	logx.L(ctx).Error("get order action record failed", zap.Error(err), zap.String("action_type", actionType), zap.String("action_key", actionKey))
	return nil, common.ErrInternalError
}

func (s *OrderService) createPendingActionRecord(ctx context.Context, actionType, actionKey string, orderID, userID int64) (*dalmodel.OrderActionRecord, *common.BizError) {
	if strings.TrimSpace(actionKey) == "" {
		return nil, nil
	}

	record := &dalmodel.OrderActionRecord{
		ID:         s.node.Generate().Int64(),
		ActionType: actionType,
		ActionKey:  actionKey,
		OrderID:    orderID,
		UserID:     userID,
		Status:     actionStatusPending,
	}
	if err := s.repo.CreateActionRecord(ctx, record); err != nil {
		if err == repository.ErrActionRecordExists {
			existing, lookupErr := s.findActionRecord(ctx, actionType, actionKey)
			if lookupErr != nil {
				return nil, lookupErr
			}
			if existing != nil {
				return existing, nil
			}
		}
		logx.L(ctx).Error("create order action record failed", zap.Error(err), zap.String("action_type", actionType), zap.String("action_key", actionKey))
		return nil, common.ErrInternalError
	}
	return record, nil
}

func (s *OrderService) markActionSucceeded(ctx context.Context, actionType, actionKey string, orderID int64) {
	if strings.TrimSpace(actionKey) == "" {
		return
	}
	if err := s.repo.MarkActionRecordSucceeded(ctx, actionType, actionKey, orderID); err != nil {
		logx.L(ctx).Error("mark order action succeeded failed", zap.Error(err), zap.String("action_type", actionType), zap.String("action_key", actionKey), zap.Int64("order_id", orderID))
	}
}

func (s *OrderService) markActionFailed(ctx context.Context, actionType, actionKey string, bizErr *common.BizError) {
	if strings.TrimSpace(actionKey) == "" {
		return
	}
	message := ""
	if bizErr != nil {
		message = bizErr.Msg
	}
	if err := s.repo.MarkActionRecordFailed(ctx, actionType, actionKey, message); err != nil {
		logx.L(ctx).Error("mark order action failed failed", zap.Error(err), zap.String("action_type", actionType), zap.String("action_key", actionKey))
	}
}

func (s *OrderService) loadOrderByActionRecord(ctx context.Context, record *dalmodel.OrderActionRecord) (*orderpb.Order, *common.BizError) {
	if record == nil || record.OrderID <= 0 {
		return nil, errno.ErrOrderIdempotencyBusy
	}
	order, err := s.repo.GetByID(ctx, record.OrderID)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return toRPCOrder(order), nil
}

func mapProductRPCError(code int32) *common.BizError {
	switch code {
	case 2020001, 2020003:
		return errno.ErrOrderProductUnavailable
	case 2020002, 2020004:
		return errno.ErrOrderSKUUnavailable
	default:
		return common.ErrServiceUnavailable
	}
}

func mapInventoryRPCError(code int32) *common.BizError {
	switch code {
	case 2050002:
		return errno.ErrOrderInsufficientStock
	case 2050006, 2050007:
		return errno.ErrOrderStateConflict
	default:
		return common.ErrServiceUnavailable
	}
}
