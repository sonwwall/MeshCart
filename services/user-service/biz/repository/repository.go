package repository

import (
	"context"
	"errors"

	dalmodel "meshcart/services/user-service/dal/model"

	"gorm.io/gorm"
)

var ErrUserNotFound = errors.New("user not found")

type UserRepository interface {
	GetByUsername(ctx context.Context, username string) (*dalmodel.User, error)
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
