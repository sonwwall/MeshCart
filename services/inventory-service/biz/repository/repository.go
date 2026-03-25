package repository

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	logx "meshcart/app/log"
	dalmodel "meshcart/services/inventory-service/dal/model"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	ErrStockNotFound            = errors.New("inventory stock not found")
	ErrStockExists              = errors.New("inventory stock already exists")
	ErrInvalidQuantity          = errors.New("invalid stock quantity")
	ErrInsufficientStock        = errors.New("insufficient stock")
	ErrStockFrozen              = errors.New("inventory stock frozen")
	ErrReservationConflict      = errors.New("inventory reservation conflict")
	ErrReservationNotFound      = errors.New("inventory reservation not found")
	ErrReservationStateConflict = errors.New("inventory reservation state conflict")
	ErrReservationTimeout       = errors.New("inventory reservation timeout")
)

type ReservationItem struct {
	SKUID    int64
	Quantity int64
}

type InventoryRepository interface {
	GetBySKUID(ctx context.Context, skuID int64) (*dalmodel.InventoryStock, error)
	ListBySKUIDs(ctx context.Context, skuIDs []int64) ([]*dalmodel.InventoryStock, error)
	CreateBatch(ctx context.Context, stocks []*dalmodel.InventoryStock) ([]*dalmodel.InventoryStock, error)
	CreateBatchWithTxBranch(ctx context.Context, branch *dalmodel.InventoryTxBranch, stocks []*dalmodel.InventoryStock) ([]*dalmodel.InventoryStock, error)
	CompensateInitWithTxBranch(ctx context.Context, branch *dalmodel.InventoryTxBranch, skuIDs []int64) error
	FreezeBySKUIDs(ctx context.Context, skuIDs []int64) ([]*dalmodel.InventoryStock, error)
	AdjustTotalStock(ctx context.Context, skuID int64, totalStock int64) (*dalmodel.InventoryStock, error)
	GetTxBranch(ctx context.Context, globalTxID, branchID, action string) (*dalmodel.InventoryTxBranch, error)
	Reserve(ctx context.Context, bizType, bizID string, items []ReservationItem) ([]*dalmodel.InventoryStock, error)
	Release(ctx context.Context, bizType, bizID string, items []ReservationItem) ([]*dalmodel.InventoryStock, error)
	ConfirmDeduct(ctx context.Context, bizType, bizID string, items []ReservationItem) ([]*dalmodel.InventoryStock, error)
}

type MySQLInventoryRepository struct {
	db           *gorm.DB
	queryTimeout time.Duration
	nextID       func() int64
}

func NewMySQLInventoryRepository(db *gorm.DB, queryTimeout time.Duration, idGenerator ...func() int64) *MySQLInventoryRepository {
	nextID := defaultInventoryRecordID
	if len(idGenerator) > 0 && idGenerator[0] != nil {
		nextID = idGenerator[0]
	}
	return &MySQLInventoryRepository{db: db, queryTimeout: queryTimeout, nextID: nextID}
}

func (r *MySQLInventoryRepository) GetBySKUID(ctx context.Context, skuID int64) (*dalmodel.InventoryStock, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	var stock dalmodel.InventoryStock
	if err := r.db.WithContext(ctx).Where("sku_id = ?", skuID).Take(&stock).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrStockNotFound
		}
		logx.L(ctx).Error("get inventory stock by sku_id failed",
			zap.Error(err),
			zap.Int64("sku_id", skuID),
		)
		return nil, err
	}
	return &stock, nil
}

func (r *MySQLInventoryRepository) ListBySKUIDs(ctx context.Context, skuIDs []int64) ([]*dalmodel.InventoryStock, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	var stocks []*dalmodel.InventoryStock
	if len(skuIDs) == 0 {
		return stocks, nil
	}
	if err := r.db.WithContext(ctx).
		Where("sku_id IN ?", skuIDs).
		Order("sku_id ASC").
		Find(&stocks).Error; err != nil {
		logx.L(ctx).Error("list inventory stocks by sku_ids failed",
			zap.Error(err),
			zap.Int64s("sku_ids", skuIDs),
		)
		return nil, err
	}
	return stocks, nil
}

func (r *MySQLInventoryRepository) CreateBatch(ctx context.Context, stocks []*dalmodel.InventoryStock) ([]*dalmodel.InventoryStock, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if len(stocks) == 0 {
		return []*dalmodel.InventoryStock{}, nil
	}
	for _, stock := range stocks {
		if stock == nil || stock.SKUID <= 0 || stock.TotalStock < 0 || stock.ReservedStock < 0 || stock.AvailableStock < 0 {
			return nil, ErrInvalidQuantity
		}
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, stock := range stocks {
			if err := tx.Create(stock).Error; err != nil {
				lowerErr := strings.ToLower(err.Error())
				if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(lowerErr, "duplicate") || strings.Contains(lowerErr, "unique constraint") {
					logx.L(ctx).Warn("create inventory stock duplicate key",
						zap.Error(err),
						zap.Int64("sku_id", stock.SKUID),
						zap.Int64("total_stock", stock.TotalStock),
					)
					return ErrStockExists
				}
				logx.L(ctx).Error("create inventory stock failed",
					zap.Error(err),
					zap.Int64("sku_id", stock.SKUID),
					zap.Int64("total_stock", stock.TotalStock),
				)
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return stocks, nil
}

func (r *MySQLInventoryRepository) CreateBatchWithTxBranch(ctx context.Context, branch *dalmodel.InventoryTxBranch, stocks []*dalmodel.InventoryStock) ([]*dalmodel.InventoryStock, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if len(stocks) == 0 {
		return []*dalmodel.InventoryStock{}, nil
	}
	for _, stock := range stocks {
		if stock == nil || stock.SKUID <= 0 || stock.TotalStock < 0 || stock.ReservedStock < 0 || stock.AvailableStock < 0 {
			return nil, ErrInvalidQuantity
		}
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if branch != nil {
			if err := tx.Create(branch).Error; err != nil {
				lowerErr := strings.ToLower(err.Error())
				if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(lowerErr, "duplicate") || strings.Contains(lowerErr, "unique constraint") {
					logx.L(ctx).Warn("create inventory tx branch duplicate key",
						zap.Error(err),
						zap.String("global_tx_id", branch.GlobalTxID),
						zap.String("branch_id", branch.BranchID),
						zap.String("action", branch.Action),
					)
					return ErrStockExists
				}
				logx.L(ctx).Error("create inventory tx branch failed",
					zap.Error(err),
					zap.String("global_tx_id", branch.GlobalTxID),
					zap.String("branch_id", branch.BranchID),
					zap.String("action", branch.Action),
				)
				return err
			}
		}
		for _, stock := range stocks {
			if err := tx.Create(stock).Error; err != nil {
				lowerErr := strings.ToLower(err.Error())
				if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(lowerErr, "duplicate") || strings.Contains(lowerErr, "unique constraint") {
					logx.L(ctx).Warn("create inventory stock with tx branch duplicate key",
						zap.Error(err),
						zap.Int64("sku_id", stock.SKUID),
						zap.String("global_tx_id", branch.GlobalTxID),
						zap.String("branch_id", branch.BranchID),
					)
					return ErrStockExists
				}
				logx.L(ctx).Error("create inventory stock with tx branch failed",
					zap.Error(err),
					zap.Int64("sku_id", stock.SKUID),
					zap.String("global_tx_id", branch.GlobalTxID),
					zap.String("branch_id", branch.BranchID),
				)
				return err
			}
		}
		if branch != nil {
			if err := tx.Model(&dalmodel.InventoryTxBranch{}).
				Where("id = ?", branch.ID).
				Updates(map[string]any{
					"status":           branch.Status,
					"payload_snapshot": branch.PayloadSnapshot,
					"error_message":    branch.ErrorMessage,
				}).Error; err != nil {
				logx.L(ctx).Error("update inventory tx branch after create failed",
					zap.Error(err),
					zap.String("global_tx_id", branch.GlobalTxID),
					zap.String("branch_id", branch.BranchID),
					zap.String("action", branch.Action),
					zap.String("status", branch.Status),
				)
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return stocks, nil
}

func (r *MySQLInventoryRepository) CompensateInitWithTxBranch(ctx context.Context, branch *dalmodel.InventoryTxBranch, skuIDs []int64) error {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if branch != nil {
			if err := tx.Create(branch).Error; err != nil {
				lowerErr := strings.ToLower(err.Error())
				if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(lowerErr, "duplicate") || strings.Contains(lowerErr, "unique constraint") {
					logx.L(ctx).Warn("create inventory compensate tx branch duplicate key",
						zap.Error(err),
						zap.String("global_tx_id", branch.GlobalTxID),
						zap.String("branch_id", branch.BranchID),
						zap.String("action", branch.Action),
					)
					return ErrStockExists
				}
				logx.L(ctx).Error("create inventory compensate tx branch failed",
					zap.Error(err),
					zap.String("global_tx_id", branch.GlobalTxID),
					zap.String("branch_id", branch.BranchID),
					zap.String("action", branch.Action),
				)
				return err
			}
		}
		if len(skuIDs) > 0 {
			if err := tx.Where("sku_id IN ?", skuIDs).Delete(&dalmodel.InventoryStock{}).Error; err != nil {
				logx.L(ctx).Error("delete inventory stocks for compensation failed",
					zap.Error(err),
					zap.Int64s("sku_ids", skuIDs),
				)
				return err
			}
		}
		if branch != nil {
			if err := tx.Model(&dalmodel.InventoryTxBranch{}).
				Where("id = ?", branch.ID).
				Updates(map[string]any{
					"status":           branch.Status,
					"payload_snapshot": branch.PayloadSnapshot,
					"error_message":    branch.ErrorMessage,
				}).Error; err != nil {
				logx.L(ctx).Error("update inventory compensate tx branch failed",
					zap.Error(err),
					zap.String("global_tx_id", branch.GlobalTxID),
					zap.String("branch_id", branch.BranchID),
					zap.String("action", branch.Action),
					zap.String("status", branch.Status),
				)
				return err
			}
		}
		return nil
	})
}

func (r *MySQLInventoryRepository) AdjustTotalStock(ctx context.Context, skuID int64, totalStock int64) (*dalmodel.InventoryStock, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if skuID <= 0 || totalStock < 0 {
		return nil, ErrInvalidQuantity
	}

	var stock dalmodel.InventoryStock
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("sku_id = ?", skuID).Take(&stock).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStockNotFound
			}
			logx.L(ctx).Error("load inventory stock for adjust failed",
				zap.Error(err),
				zap.Int64("sku_id", skuID),
				zap.Int64("target_total_stock", totalStock),
			)
			return err
		}
		if totalStock < stock.ReservedStock {
			return ErrInvalidQuantity
		}
		available := totalStock - stock.ReservedStock
		if err := tx.Model(&dalmodel.InventoryStock{}).
			Where("id = ?", stock.ID).
			Updates(map[string]any{
				"total_stock":     totalStock,
				"available_stock": available,
				"version":         stock.Version + 1,
			}).Error; err != nil {
			logx.L(ctx).Error("adjust inventory stock failed",
				zap.Error(err),
				zap.Int64("sku_id", skuID),
				zap.Int64("target_total_stock", totalStock),
				zap.Int64("reserved_stock", stock.ReservedStock),
			)
			return err
		}
		if err := tx.Where("id = ?", stock.ID).Take(&stock).Error; err != nil {
			logx.L(ctx).Error("reload inventory stock after adjust failed",
				zap.Error(err),
				zap.Int64("sku_id", skuID),
			)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &stock, nil
}

func (r *MySQLInventoryRepository) FreezeBySKUIDs(ctx context.Context, skuIDs []int64) ([]*dalmodel.InventoryStock, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	if len(skuIDs) == 0 {
		return []*dalmodel.InventoryStock{}, nil
	}
	for _, skuID := range skuIDs {
		if skuID <= 0 {
			return nil, ErrInvalidQuantity
		}
	}

	var stocks []*dalmodel.InventoryStock
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("sku_id IN ?", skuIDs).Order("sku_id ASC").Find(&stocks).Error; err != nil {
			logx.L(ctx).Error("load inventory stocks for freeze failed",
				zap.Error(err),
				zap.Int64s("sku_ids", skuIDs),
			)
			return err
		}
		if len(stocks) == 0 {
			return nil
		}
		if err := tx.Model(&dalmodel.InventoryStock{}).
			Where("sku_id IN ?", skuIDs).
			Updates(map[string]any{"status": 0}).Error; err != nil {
			logx.L(ctx).Error("freeze inventory stocks update failed",
				zap.Error(err),
				zap.Int64s("sku_ids", skuIDs),
			)
			return err
		}
		if err := tx.Where("sku_id IN ?", skuIDs).Order("sku_id ASC").Find(&stocks).Error; err != nil {
			logx.L(ctx).Error("reload inventory stocks after freeze failed",
				zap.Error(err),
				zap.Int64s("sku_ids", skuIDs),
			)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return stocks, nil
}

func withQueryTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, timeout)
}

func (r *MySQLInventoryRepository) GetTxBranch(ctx context.Context, globalTxID, branchID, action string) (*dalmodel.InventoryTxBranch, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	var branch dalmodel.InventoryTxBranch
	err := r.db.WithContext(ctx).
		Where("global_tx_id = ? AND branch_id = ? AND action = ?", globalTxID, branchID, action).
		Take(&branch).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		logx.L(ctx).Error("get inventory tx branch failed",
			zap.Error(err),
			zap.String("global_tx_id", globalTxID),
			zap.String("branch_id", branchID),
			zap.String("action", action),
		)
		return nil, err
	}
	return &branch, nil
}

const (
	reservationStatusReserved  = "reserved"
	reservationStatusReleased  = "released"
	reservationStatusConfirmed = "confirmed"
)

func (r *MySQLInventoryRepository) Reserve(ctx context.Context, bizType, bizID string, items []ReservationItem) ([]*dalmodel.InventoryStock, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	orderedItems := sortReservationItems(items)
	skuIDs, err := validateReservationItems(orderedItems)
	if err != nil {
		return nil, err
	}

	var finalStocks []*dalmodel.InventoryStock
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing map[int64]*dalmodel.InventoryReservation
		for _, item := range orderedItems {
			payload := buildReservationPayload(bizType, bizID, item)
			if err := tx.Create(&dalmodel.InventoryReservation{
				ID:              r.nextID(),
				BizType:         bizType,
				BizID:           bizID,
				SKUID:           item.SKUID,
				Quantity:        item.Quantity,
				Status:          reservationStatusReserved,
				PayloadSnapshot: payload,
			}).Error; err != nil {
				if normalizeDuplicateError(err) == ErrReservationConflict {
					if existing == nil {
						existing, err = loadReservations(tx, bizType, bizID, skuIDs)
						if err != nil {
							return err
						}
					}
					reservation := existing[item.SKUID]
					if reservation == nil {
						logx.L(ctx).Warn("inventory reserve duplicate key without existing reservation",
							zap.String("biz_type", bizType),
							zap.String("biz_id", bizID),
							zap.Int64("sku_id", item.SKUID),
						)
						return ErrReservationConflict
					}
					if reservation.Quantity != item.Quantity {
						logx.L(ctx).Warn("inventory reserve rejected by quantity mismatch",
							zap.String("biz_type", bizType),
							zap.String("biz_id", bizID),
							zap.Int64("sku_id", item.SKUID),
							zap.Int64("expected_quantity", reservation.Quantity),
							zap.Int64("actual_quantity", item.Quantity),
						)
						return ErrReservationConflict
					}
					if reservation.Status == reservationStatusReserved {
						continue
					}
					logx.L(ctx).Warn("inventory reserve rejected by reservation state",
						zap.String("biz_type", bizType),
						zap.String("biz_id", bizID),
						zap.Int64("sku_id", item.SKUID),
						zap.String("reservation_status", reservation.Status),
					)
					return ErrReservationStateConflict
				}
				logx.L(ctx).Error("create inventory reservation for reserve failed",
					zap.Error(err),
					zap.String("biz_type", bizType),
					zap.String("biz_id", bizID),
					zap.Int64("sku_id", item.SKUID),
					zap.Int64("quantity", item.Quantity),
				)
				return err
			}

			result := tx.Model(&dalmodel.InventoryStock{}).
				Where("sku_id = ? AND status = ? AND available_stock >= ?", item.SKUID, 1, item.Quantity).
				Updates(map[string]any{
					"reserved_stock":  gorm.Expr("reserved_stock + ?", item.Quantity),
					"available_stock": gorm.Expr("available_stock - ?", item.Quantity),
					"version":         gorm.Expr("version + 1"),
				})
			if result.Error != nil {
				logx.L(ctx).Error("inventory reserve stock update failed",
					zap.Error(result.Error),
					zap.String("biz_type", bizType),
					zap.String("biz_id", bizID),
					zap.Int64("sku_id", item.SKUID),
					zap.Int64("quantity", item.Quantity),
				)
				return result.Error
			}
			if result.RowsAffected == 0 {
				return classifyReserveFailure(tx, item)
			}
		}
		finalStocks, err = loadStocksBySKUIDs(tx, skuIDs)
		return err
	})
	if err != nil {
		return nil, normalizeReservationError(err)
	}
	return finalStocks, nil
}

func (r *MySQLInventoryRepository) Release(ctx context.Context, bizType, bizID string, items []ReservationItem) ([]*dalmodel.InventoryStock, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	orderedItems := sortReservationItems(items)
	skuIDs, err := validateReservationItems(orderedItems)
	if err != nil {
		return nil, err
	}

	var finalStocks []*dalmodel.InventoryStock
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, err := loadReservations(tx, bizType, bizID, skuIDs)
		if err != nil {
			return err
		}
		for _, item := range orderedItems {
			reservation := existing[item.SKUID]
			if reservation == nil {
				payload := buildReservationPayload(bizType, bizID, item)
				if err := tx.Create(&dalmodel.InventoryReservation{
					ID:              r.nextID(),
					BizType:         bizType,
					BizID:           bizID,
					SKUID:           item.SKUID,
					Quantity:        item.Quantity,
					Status:          reservationStatusReleased,
					PayloadSnapshot: payload,
				}).Error; err != nil {
					logx.L(ctx).Error("create inventory release placeholder reservation failed",
						zap.Error(err),
						zap.String("biz_type", bizType),
						zap.String("biz_id", bizID),
						zap.Int64("sku_id", item.SKUID),
						zap.Int64("quantity", item.Quantity),
					)
					return normalizeDuplicateError(err)
				}
				continue
			}
			if reservation.Quantity != item.Quantity {
				logx.L(ctx).Warn("inventory release rejected by quantity mismatch",
					zap.String("biz_type", bizType),
					zap.String("biz_id", bizID),
					zap.Int64("sku_id", item.SKUID),
					zap.Int64("expected_quantity", reservation.Quantity),
					zap.Int64("actual_quantity", item.Quantity),
				)
				return ErrReservationConflict
			}
			switch reservation.Status {
			case reservationStatusReleased:
				continue
			case reservationStatusConfirmed:
				logx.L(ctx).Warn("inventory release rejected by confirmed reservation",
					zap.String("biz_type", bizType),
					zap.String("biz_id", bizID),
					zap.Int64("sku_id", item.SKUID),
				)
				return ErrReservationStateConflict
			case reservationStatusReserved:
				result := tx.Model(&dalmodel.InventoryStock{}).
					Where("sku_id = ? AND reserved_stock >= ?", item.SKUID, item.Quantity).
					Updates(map[string]any{
						"reserved_stock":  gorm.Expr("reserved_stock - ?", item.Quantity),
						"available_stock": gorm.Expr("available_stock + ?", item.Quantity),
						"version":         gorm.Expr("version + 1"),
					})
				if result.Error != nil {
					logx.L(ctx).Error("inventory release stock update failed",
						zap.Error(result.Error),
						zap.String("biz_type", bizType),
						zap.String("biz_id", bizID),
						zap.Int64("sku_id", item.SKUID),
						zap.Int64("quantity", item.Quantity),
					)
					return result.Error
				}
				if result.RowsAffected == 0 {
					return ErrReservationStateConflict
				}
				if err := tx.Model(&dalmodel.InventoryReservation{}).
					Where("id = ?", reservation.ID).
					Updates(map[string]any{
						"status":           reservationStatusReleased,
						"payload_snapshot": buildReservationPayload(bizType, bizID, item),
					}).Error; err != nil {
					logx.L(ctx).Error("update inventory reservation to released failed",
						zap.Error(err),
						zap.String("biz_type", bizType),
						zap.String("biz_id", bizID),
						zap.Int64("sku_id", item.SKUID),
					)
					return err
				}
			default:
				return ErrReservationStateConflict
			}
		}
		finalStocks, err = loadStocksBySKUIDs(tx, skuIDs)
		return err
	})
	if err != nil {
		return nil, err
	}
	return finalStocks, nil
}

func (r *MySQLInventoryRepository) ConfirmDeduct(ctx context.Context, bizType, bizID string, items []ReservationItem) ([]*dalmodel.InventoryStock, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	orderedItems := sortReservationItems(items)
	skuIDs, err := validateReservationItems(orderedItems)
	if err != nil {
		return nil, err
	}

	var finalStocks []*dalmodel.InventoryStock
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, err := loadReservations(tx, bizType, bizID, skuIDs)
		if err != nil {
			return err
		}
		for _, item := range orderedItems {
			reservation := existing[item.SKUID]
			if reservation == nil {
				logx.L(ctx).Warn("inventory confirm deduct rejected by missing reservation",
					zap.String("biz_type", bizType),
					zap.String("biz_id", bizID),
					zap.Int64("sku_id", item.SKUID),
				)
				return ErrReservationNotFound
			}
			if reservation.Quantity != item.Quantity {
				logx.L(ctx).Warn("inventory confirm deduct rejected by quantity mismatch",
					zap.String("biz_type", bizType),
					zap.String("biz_id", bizID),
					zap.Int64("sku_id", item.SKUID),
					zap.Int64("expected_quantity", reservation.Quantity),
					zap.Int64("actual_quantity", item.Quantity),
				)
				return ErrReservationConflict
			}
			switch reservation.Status {
			case reservationStatusConfirmed:
				continue
			case reservationStatusReleased:
				logx.L(ctx).Warn("inventory confirm deduct rejected by released reservation",
					zap.String("biz_type", bizType),
					zap.String("biz_id", bizID),
					zap.Int64("sku_id", item.SKUID),
				)
				return ErrReservationStateConflict
			case reservationStatusReserved:
				result := tx.Model(&dalmodel.InventoryStock{}).
					Where("sku_id = ? AND reserved_stock >= ? AND total_stock >= ?", item.SKUID, item.Quantity, item.Quantity).
					Updates(map[string]any{
						"total_stock":    gorm.Expr("total_stock - ?", item.Quantity),
						"reserved_stock": gorm.Expr("reserved_stock - ?", item.Quantity),
						"version":        gorm.Expr("version + 1"),
					})
				if result.Error != nil {
					logx.L(ctx).Error("inventory confirm deduct stock update failed",
						zap.Error(result.Error),
						zap.String("biz_type", bizType),
						zap.String("biz_id", bizID),
						zap.Int64("sku_id", item.SKUID),
						zap.Int64("quantity", item.Quantity),
					)
					return result.Error
				}
				if result.RowsAffected == 0 {
					return ErrReservationStateConflict
				}
				if err := tx.Model(&dalmodel.InventoryReservation{}).
					Where("id = ?", reservation.ID).
					Updates(map[string]any{
						"status":           reservationStatusConfirmed,
						"payload_snapshot": buildReservationPayload(bizType, bizID, item),
					}).Error; err != nil {
					logx.L(ctx).Error("update inventory reservation to confirmed failed",
						zap.Error(err),
						zap.String("biz_type", bizType),
						zap.String("biz_id", bizID),
						zap.Int64("sku_id", item.SKUID),
					)
					return err
				}
			default:
				return ErrReservationStateConflict
			}
		}
		finalStocks, err = loadStocksBySKUIDs(tx, skuIDs)
		return err
	})
	if err != nil {
		return nil, err
	}
	return finalStocks, nil
}

func validateReservationItems(items []ReservationItem) ([]int64, error) {
	if len(items) == 0 {
		return nil, ErrInvalidQuantity
	}
	seen := make(map[int64]struct{}, len(items))
	skuIDs := make([]int64, 0, len(items))
	for _, item := range items {
		if item.SKUID <= 0 || item.Quantity <= 0 {
			return nil, ErrInvalidQuantity
		}
		if _, ok := seen[item.SKUID]; ok {
			return nil, ErrReservationConflict
		}
		seen[item.SKUID] = struct{}{}
		skuIDs = append(skuIDs, item.SKUID)
	}
	return skuIDs, nil
}

func sortReservationItems(items []ReservationItem) []ReservationItem {
	if len(items) <= 1 {
		copied := make([]ReservationItem, len(items))
		copy(copied, items)
		return copied
	}
	copied := make([]ReservationItem, len(items))
	copy(copied, items)
	sort.Slice(copied, func(i, j int) bool {
		return copied[i].SKUID < copied[j].SKUID
	})
	return copied
}

func loadReservations(tx *gorm.DB, bizType, bizID string, skuIDs []int64) (map[int64]*dalmodel.InventoryReservation, error) {
	result := make(map[int64]*dalmodel.InventoryReservation, len(skuIDs))
	if len(skuIDs) == 0 {
		return result, nil
	}
	var reservations []*dalmodel.InventoryReservation
	if err := tx.Where("biz_type = ? AND biz_id = ? AND sku_id IN ?", bizType, bizID, skuIDs).Find(&reservations).Error; err != nil {
		return nil, err
	}
	for _, reservation := range reservations {
		result[reservation.SKUID] = reservation
	}
	return result, nil
}

func loadStocksBySKUIDs(tx *gorm.DB, skuIDs []int64) ([]*dalmodel.InventoryStock, error) {
	if len(skuIDs) == 0 {
		return []*dalmodel.InventoryStock{}, nil
	}
	var stocks []*dalmodel.InventoryStock
	if err := tx.Where("sku_id IN ?", skuIDs).Order("sku_id ASC").Find(&stocks).Error; err != nil {
		return nil, err
	}
	return stocks, nil
}

func classifyReserveFailure(tx *gorm.DB, item ReservationItem) error {
	var stock dalmodel.InventoryStock
	if err := tx.Where("sku_id = ?", item.SKUID).Take(&stock).Error; err != nil {
		if isReservationTimeoutErr(err) {
			return ErrReservationTimeout
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrStockNotFound
		}
		return err
	}
	if stock.Status != 1 {
		return ErrStockFrozen
	}
	if stock.AvailableStock < item.Quantity {
		return ErrInsufficientStock
	}
	return ErrReservationStateConflict
}

func normalizeDuplicateError(err error) error {
	if err == nil {
		return nil
	}
	lowerErr := strings.ToLower(err.Error())
	if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(lowerErr, "duplicate") || strings.Contains(lowerErr, "unique constraint") {
		return ErrReservationConflict
	}
	return err
}

func normalizeReservationError(err error) error {
	if isReservationTimeoutErr(err) {
		return ErrReservationTimeout
	}
	return normalizeDuplicateError(err)
}

func isReservationTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	lowerErr := strings.ToLower(err.Error())
	return strings.Contains(lowerErr, "context deadline exceeded") ||
		strings.Contains(lowerErr, "lock wait timeout exceeded") ||
		strings.Contains(lowerErr, "deadlock found when trying to get lock")
}

func buildReservationPayload(bizType, bizID string, item ReservationItem) string {
	return `{"biz_type":"` + bizType + `","biz_id":"` + bizID + `","sku_id":` + int64ToString(item.SKUID) + `,"quantity":` + int64ToString(item.Quantity) + `}`
}

func int64ToString(value int64) string {
	return strconv.FormatInt(value, 10)
}

func defaultInventoryRecordID() int64 {
	return time.Now().UnixNano()
}
