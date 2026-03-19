package db

import (
	"errors"

	"github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(dsn, sourceURL string) error {
	gdb, err := NewMySQL(dsn)
	if err != nil {
		return err
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	driver, err := migratemysql.WithInstance(sqlDB, &migratemysql.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithDatabaseInstance(sourceURL, "mysql", driver)
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}
