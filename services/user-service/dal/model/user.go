package model

import "time"

type User struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	Username  string    `gorm:"column:username;type:varchar(64);uniqueIndex:uk_username;not null"`
	Password  string    `gorm:"column:password;type:varchar(255);not null"`
	Role      string    `gorm:"column:role;type:varchar(32);not null;default:user;index:idx_role"`
	IsLocked  bool      `gorm:"column:is_locked;type:tinyint(1);not null;default:0"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (User) TableName() string {
	return "users"
}
