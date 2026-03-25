package db

import (
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func NewMySQL(dsn string, pool PoolConfig) (*gorm.DB, error) {
	gdb, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}

	if pool.MaxOpenConns <= 0 {
		pool.MaxOpenConns = 20
	}
	if pool.MaxIdleConns <= 0 {
		pool.MaxIdleConns = 10
	}
	if pool.ConnMaxLifetime <= 0 {
		pool.ConnMaxLifetime = 30 * time.Minute
	}

	sqlDB.SetMaxOpenConns(pool.MaxOpenConns)
	sqlDB.SetMaxIdleConns(pool.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(pool.ConnMaxLifetime)

	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	return gdb, nil
}
