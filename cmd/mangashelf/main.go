package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/TotallyLegitimateOrg/Mangashelf/internal/auth"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/buildinfo"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/config"
	dbpkg "github.com/TotallyLegitimateOrg/Mangashelf/internal/db"
	httpapi "github.com/TotallyLegitimateOrg/Mangashelf/internal/http"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/services"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/store"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}
	if cfg.JWTSecretGenerated {
		logger.Warn("MANGASHELF_JWT_SECRET is not set; generated an ephemeral secret and JWT sessions will be invalidated on restart")
	}
	services.ConfigureCatboxUserHash(cfg.CatboxUserHash)

	database, err := dbpkg.Open(ctx, cfg)
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	defer database.SQL.Close()

	dataStore := store.New(database.SQL, logger)
	authManager := auth.New(cfg.JWTSecret, dataStore)
	server, err := httpapi.New(cfg, logger, dataStore, authManager)
	if err != nil {
		logger.Error("build server", "error", err)
		os.Exit(1)
	}
	info := buildinfo.Current()

	// Start background sync scheduler
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				dataStore.RunSyncDueSources(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()

	logger.Info(
		"starting Mangashelf",
		"version", info.Version,
		"commit", info.Commit,
		"builtAt", info.BuiltAt,
		"port", cfg.HTTPPort,
		"databasePath", cfg.DatabasePath,
		"devWebURL", cfg.DevWebURL,
		"devExtensionURL", cfg.DevExtensionURL,
	)
	if err := http.ListenAndServe(":"+cfg.HTTPPort, server.Handler()); err != nil {
		logger.Error("server exited", "error", err)
		os.Exit(1)
	}
}
