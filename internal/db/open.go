package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/TotallyLegitimateOrg/Mangashelf/internal/config"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/db/gen"

	_ "modernc.org/sqlite"
)

type Database struct {
	SQL     *sql.DB
	Queries *gen.Queries
}

func Open(ctx context.Context, cfg config.Config) (*Database, error) {
	if err := cfg.EnsureDatabaseDir(); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	conn, err := sql.Open("sqlite", cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	conn.SetConnMaxLifetime(30 * time.Minute)
	conn.SetMaxIdleConns(5)
	conn.SetMaxOpenConns(10)

	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON;"); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := applyMigrations(ctx, conn); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	return &Database{
		SQL:     conn,
		Queries: gen.New(conn),
	}, nil
}
