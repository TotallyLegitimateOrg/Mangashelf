package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
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

	conn, err := sql.Open("sqlite", sqliteDSN(cfg.DatabasePath))
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

	if err := applyMigrations(ctx, conn); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	return &Database{
		SQL:     conn,
		Queries: gen.New(conn),
	}, nil
}

func sqliteDSN(databasePath string) string {
	pragmas := []string{
		"busy_timeout(5000)",
		"foreign_keys(ON)",
	}
	if databasePath != ":memory:" && !strings.Contains(databasePath, "mode=memory") {
		pragmas = append(pragmas, "journal_mode(WAL)")
	}

	separator := "?"
	if strings.Contains(databasePath, "?") {
		separator = "&"
	}
	params := make([]string, 0, len(pragmas))
	for _, pragma := range pragmas {
		params = append(params, "_pragma="+url.QueryEscape(pragma))
	}
	return databasePath + separator + strings.Join(params, "&")
}
