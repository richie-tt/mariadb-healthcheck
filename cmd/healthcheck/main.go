// Package main is the entry point for the healthcheck command.
// It parses the environment variables and starts the HTTP server.
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

var (
	// Version is the application version
	Version = ""
	// BuildDate is the date the application was built
	BuildDate = ""
	// Commit is the git commit hash the application was built from
	Commit = ""
)

func main() {
	if err := run(); err != nil {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}
}

func (c config) getLogLevel() (slog.Level, error) {
	switch c.LogLevel {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level: %s", c.LogLevel)
	}
}

func setupServer(config config) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", config.healthHandler)

	return &http.Server{
		Addr:         fmt.Sprintf(":%d", config.HealthPort),
		Handler:      mux,
		ReadTimeout:  httpReadTimeout,
		WriteTimeout: httpWriteTimeout,
	}
}

func run() error {
	slog.Info(
		"starting healthcheck",
		"version", Version,
		"commit", Commit,
		"build_date", BuildDate,
	)

	env := getEnv()

	config, err := env.parseEnv()
	if err != nil {
		return fmt.Errorf("failed to parse environment: %w", err)
	}

	db, err := config.Connection.ConnectDB()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	config.DBInterface = db

	server := setupServer(*config)

	defer server.Close()

	slog.Info(
		"starting health check server",
		"port", config.HealthPort,
	)

	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}
