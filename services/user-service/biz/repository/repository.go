package repository

import (
	"context"
	"database/sql"
	"errors"

	dalmodel "meshcart/services/user-service/dal/model"
)

var ErrUserNotFound = errors.New("user not found")

type UserRepository interface {
	GetByUsername(ctx context.Context, username string) (*dalmodel.User, error)
}

type MySQLUserRepository struct {
	db *sql.DB
}

func NewMySQLUserRepository(db *sql.DB) *MySQLUserRepository {
	return &MySQLUserRepository{db: db}
}

func (r *MySQLUserRepository) GetByUsername(ctx context.Context, username string) (*dalmodel.User, error) {
	const query = `
		SELECT id, username, password, is_locked
		FROM users
		WHERE username = ?
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, username)
	user := &dalmodel.User{}
	if err := row.Scan(&user.ID, &user.Username, &user.Password, &user.IsLocked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}
