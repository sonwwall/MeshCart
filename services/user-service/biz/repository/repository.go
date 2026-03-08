package repository

import (
	"context"
	"errors"

	dalmodel "meshcart/services/user-service/dal/model"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

var ErrUserNotFound = errors.New("user not found")
var ErrUserAlreadyExists = errors.New("user already exists")

type UserRepository interface {
	GetByUsername(ctx context.Context, username string) (*dalmodel.User, error)
	Create(ctx context.Context, user *dalmodel.User) error
}

type MySQLUserRepository struct {
	db *gorm.DB
}

func NewMySQLUserRepository(db *gorm.DB) *MySQLUserRepository {
	return &MySQLUserRepository{db: db}
}

func (r *MySQLUserRepository) GetByUsername(ctx context.Context, username string) (*dalmodel.User, error) {
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

func (r *MySQLUserRepository) Create(ctx context.Context, user *dalmodel.User) error {
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
