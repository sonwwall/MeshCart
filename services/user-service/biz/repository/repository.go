package repository

import (
	"context"
	"errors"
	"time"

	logx "meshcart/app/log"
	dalmodel "meshcart/services/user-service/dal/model"

	"github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var ErrUserNotFound = errors.New("user not found")
var ErrUserAlreadyExists = errors.New("user already exists")

type UserRepository interface {
	GetByUsername(ctx context.Context, username string) (*dalmodel.User, error)
	GetByID(ctx context.Context, userID int64) (*dalmodel.User, error)
	Count(ctx context.Context) (int64, error)
	CountByRole(ctx context.Context, role string) (int64, error)
	Create(ctx context.Context, user *dalmodel.User) error
	UpdateRole(ctx context.Context, userID int64, role string) error
}

type MySQLUserRepository struct {
	db           *gorm.DB
	queryTimeout time.Duration
}

func NewMySQLUserRepository(db *gorm.DB, queryTimeout time.Duration) *MySQLUserRepository {
	return &MySQLUserRepository{db: db, queryTimeout: queryTimeout}
}

func (r *MySQLUserRepository) GetByUsername(ctx context.Context, username string) (*dalmodel.User, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	user := &dalmodel.User{}
	err := r.db.WithContext(ctx).
		Where("username = ?", username).
		Limit(1).
		First(user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		logx.L(ctx).Error("get user by username failed",
			zap.Error(err),
			zap.String("username", username),
		)
		return nil, err
	}
	return user, nil
}

func (r *MySQLUserRepository) GetByID(ctx context.Context, userID int64) (*dalmodel.User, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	user := &dalmodel.User{}
	err := r.db.WithContext(ctx).
		Where("id = ?", userID).
		Limit(1).
		First(user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		logx.L(ctx).Error("get user by id failed",
			zap.Error(err),
			zap.Int64("user_id", userID),
		)
		return nil, err
	}
	return user, nil
}

func (r *MySQLUserRepository) Count(ctx context.Context) (int64, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	var total int64
	if err := r.db.WithContext(ctx).Model(&dalmodel.User{}).Count(&total).Error; err != nil {
		logx.L(ctx).Error("count users failed", zap.Error(err))
		return 0, err
	}
	return total, nil
}

func (r *MySQLUserRepository) CountByRole(ctx context.Context, role string) (int64, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	var total int64
	if err := r.db.WithContext(ctx).Model(&dalmodel.User{}).Where("role = ?", role).Count(&total).Error; err != nil {
		logx.L(ctx).Error("count users by role failed",
			zap.Error(err),
			zap.String("role", role),
		)
		return 0, err
	}
	return total, nil
}

func (r *MySQLUserRepository) Create(ctx context.Context, user *dalmodel.User) error {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	err := r.db.WithContext(ctx).Create(user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			logx.L(ctx).Warn("create user duplicate key",
				zap.Error(err),
				zap.String("username", user.Username),
			)
			return ErrUserAlreadyExists
		}
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			logx.L(ctx).Warn("create user duplicate key",
				zap.Error(err),
				zap.String("username", user.Username),
			)
			return ErrUserAlreadyExists
		}
		logx.L(ctx).Error("create user failed",
			zap.Error(err),
			zap.String("username", user.Username),
			zap.String("role", user.Role),
		)
		return err
	}
	return nil
}

func (r *MySQLUserRepository) UpdateRole(ctx context.Context, userID int64, role string) error {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	result := r.db.WithContext(ctx).Model(&dalmodel.User{}).Where("id = ?", userID).Update("role", role)
	if result.Error != nil {
		logx.L(ctx).Error("update user role failed",
			zap.Error(result.Error),
			zap.Int64("user_id", userID),
			zap.String("role", role),
		)
		return result.Error
	}
	if result.RowsAffected == 0 {
		logx.L(ctx).Warn("update user role missed user",
			zap.Int64("user_id", userID),
			zap.String("role", role),
		)
		return ErrUserNotFound
	}
	return nil
}

func withQueryTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}
