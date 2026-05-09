package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	dbmigrations "github.com/TotallyLegitimateOrg/Mangashelf/db"

	"github.com/pressly/goose/v3"
)

func applyMigrations(ctx context.Context, conn *sql.DB) error {
	migrationFS, err := fs.Sub(dbmigrations.FS, "migrations")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, migrationFS)
	if err != nil {
		return fmt.Errorf("create goose provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("run goose migrations: %w", err)
	}
	return nil
}
