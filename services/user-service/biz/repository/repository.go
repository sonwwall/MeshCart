package repository

import (
	"context"
	"errors"
	"time"

	dalmodel "meshcart/services/user-service/dal/model"

	"github.com/go-sql-driver/mysql"
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
		return nil, err
	}
	return user, nil
}

func (r *MySQLUserRepository) Count(ctx context.Context) (int64, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	var total int64
	if err := r.db.WithContext(ctx).Model(&dalmodel.User{}).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *MySQLUserRepository) CountByRole(ctx context.Context, role string) (int64, error) {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	var total int64
	if err := r.db.WithContext(ctx).Model(&dalmodel.User{}).Where("role = ?", role).Count(&total).Error; err != nil {
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
			return ErrUserAlreadyExists
		}
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return ErrUserAlreadyExists
		}
		return err
	}
	return nil
}

func (r *MySQLUserRepository) UpdateRole(ctx context.Context, userID int64, role string) error {
	ctx, cancel := withQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	result := r.db.WithContext(ctx).Model(&dalmodel.User{}).Where("id = ?", userID).Update("role", role)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
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
