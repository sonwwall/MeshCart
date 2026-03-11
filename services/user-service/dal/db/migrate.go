package db

import (
	"database/sql"
	"errors"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(dsn, sourceURL string) error {
	migrationDSN, err := withMySQLMultiStatements(dsn)
	if err != nil {
		return err
	}

	// Use an isolated DB connection for migrations, so closing migrate resources
	// does not affect GORM business connections.
	sqlDB, err := sql.Open("mysql", migrationDSN)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	driver, err := migratemysql.WithInstance(sqlDB, &migratemysql.Config{
		MigrationsTable: "schema_migrations",
	})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithDatabaseInstance(sourceURL, "mysql", driver)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

func withMySQLMultiStatements(dsn string) (string, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return "", err
	}
	cfg.MultiStatements = true
	return cfg.FormatDSN(), nil
}
