package repository

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	dalmodel "meshcart/services/inventory-service/dal/model"

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
					return ErrStockExists
				}
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
					return ErrStockExists
				}
				return err
			}
		}
		for _, stock := range stocks {
			if err := tx.Create(stock).Error; err != nil {
				lowerErr := strings.ToLower(err.Error())
				if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(lowerErr, "duplicate") || strings.Contains(lowerErr, "unique constraint") {
					return ErrStockExists
				}
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
					return ErrStockExists
				}
				return err
			}
		}
		if len(skuIDs) > 0 {
			if err := tx.Where("sku_id IN ?", skuIDs).Delete(&dalmodel.InventoryStock{}).Error; err != nil {
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
			return err
		}
		return tx.Where("id = ?", stock.ID).Take(&stock).Error
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
			return err
		}
		if len(stocks) == 0 {
			return nil
		}
		if err := tx.Model(&dalmodel.InventoryStock{}).
			Where("sku_id IN ?", skuIDs).
			Updates(map[string]any{"status": 0}).Error; err != nil {
			return err
		}
		return tx.Where("sku_id IN ?", skuIDs).Order("sku_id ASC").Find(&stocks).Error
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

	skuIDs, err := validateReservationItems(items)
	if err != nil {
		return nil, err
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, err := loadReservations(tx, bizType, bizID, skuIDs)
		if err != nil {
			return err
		}
		for _, item := range items {
			reservation := existing[item.SKUID]
			if reservation != nil {
				if reservation.Quantity != item.Quantity {
					return ErrReservationConflict
				}
				if reservation.Status == reservationStatusReserved {
					continue
				}
				return ErrReservationStateConflict
			}

			result := tx.Model(&dalmodel.InventoryStock{}).
				Where("sku_id = ? AND status = ? AND available_stock >= ?", item.SKUID, 1, item.Quantity).
				Updates(map[string]any{
					"reserved_stock":  gorm.Expr("reserved_stock + ?", item.Quantity),
					"available_stock": gorm.Expr("available_stock - ?", item.Quantity),
					"version":         gorm.Expr("version + 1"),
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return classifyReserveFailure(tx, item)
			}

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
				return normalizeDuplicateError(err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.ListBySKUIDs(ctx, skuIDs)
}

func (r *MySQLInventoryRepository) Release(ctx context.Context, bizType, bizID string, items []ReservationItem) ([]*dalmodel.InventoryStock, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	skuIDs, err := validateReservationItems(items)
	if err != nil {
		return nil, err
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, err := loadReservations(tx, bizType, bizID, skuIDs)
		if err != nil {
			return err
		}
		for _, item := range items {
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
					return normalizeDuplicateError(err)
				}
				continue
			}
			if reservation.Quantity != item.Quantity {
				return ErrReservationConflict
			}
			switch reservation.Status {
			case reservationStatusReleased:
				continue
			case reservationStatusConfirmed:
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
					return err
				}
			default:
				return ErrReservationStateConflict
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.ListBySKUIDs(ctx, skuIDs)
}

func (r *MySQLInventoryRepository) ConfirmDeduct(ctx context.Context, bizType, bizID string, items []ReservationItem) ([]*dalmodel.InventoryStock, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	skuIDs, err := validateReservationItems(items)
	if err != nil {
		return nil, err
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, err := loadReservations(tx, bizType, bizID, skuIDs)
		if err != nil {
			return err
		}
		for _, item := range items {
			reservation := existing[item.SKUID]
			if reservation == nil {
				return ErrReservationNotFound
			}
			if reservation.Quantity != item.Quantity {
				return ErrReservationConflict
			}
			switch reservation.Status {
			case reservationStatusConfirmed:
				continue
			case reservationStatusReleased:
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
					return err
				}
			default:
				return ErrReservationStateConflict
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.ListBySKUIDs(ctx, skuIDs)
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

func classifyReserveFailure(tx *gorm.DB, item ReservationItem) error {
	var stock dalmodel.InventoryStock
	if err := tx.Where("sku_id = ?", item.SKUID).Take(&stock).Error; err != nil {
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

func buildReservationPayload(bizType, bizID string, item ReservationItem) string {
	return `{"biz_type":"` + bizType + `","biz_id":"` + bizID + `","sku_id":` + int64ToString(item.SKUID) + `,"quantity":` + int64ToString(item.Quantity) + `}`
}

func int64ToString(value int64) string {
	return strconv.FormatInt(value, 10)
}

func defaultInventoryRecordID() int64 {
	return time.Now().UnixNano()
}
