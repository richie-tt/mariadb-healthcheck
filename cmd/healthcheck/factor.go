package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/richie-tt/mariadb-healthcheck/internal/mariadb"
)

// or returns value when it is non-empty, otherwise fallback.
func or(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}

// intOr parses value as an int; returns fallback when value is empty.
func intOr(value string, fallback int) (int, error) {
	if value == "" {
		return fallback, nil
	}

	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid integer %q: %w", value, err)
	}

	return n, nil
}

// boolOr parses value as a bool; returns fallback when value is empty.
func boolOr(value string, fallback bool) (bool, error) {
	if value == "" {
		return fallback, nil
	}

	b, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid bool %q: %w", value, err)
	}

	return b, nil
}

func getEnv() environment {
	return environment{
		Connection: mariadb.Connection{
			Database: os.Getenv(dbName),
			Driver:   "mysql",
			Host:     os.Getenv(dbHost),
			Password: os.Getenv(dbPassword),
			Port:     os.Getenv(dbPort),
			User:     os.Getenv(dbUser),
		},
		DeleteRow:  os.Getenv(deleteRow),
		HealthPort: os.Getenv(healthPort),
		LogLevel:   os.Getenv(logLevel),
	}
}

func (e environment) parseEnv() (*config, error) {
	if e.Connection.Password == "" {
		return nil, fmt.Errorf("DB_PASSWORD environment variable is required")
	}

	cfg := config{
		Connection: mariadb.Connection{
			Driver:   "mysql",
			Database: or(e.Connection.Database, defaultDBName),
			Host:     or(e.Connection.Host, defaultDBHost),
			Password: e.Connection.Password,
			Port:     or(e.Connection.Port, defaultDBPort),
			User:     or(e.Connection.User, defaultDBUser),
		},
		LogLevel: or(e.LogLevel, "info"),
	}

	level, err := cfg.getLogLevel()
	if err != nil {
		slog.Error(
			"failed to get log level, available levels: debug, info, warn, error",
			"error", err,
		)

		return nil, fmt.Errorf("failed to parse the log level: %w", err)
	}

	slog.SetDefault(
		slog.New(
			slog.NewTextHandler(
				os.Stdout,
				&slog.HandlerOptions{
					Level: level,
				},
			),
		),
	)

	port, err := intOr(e.HealthPort, defaultHTTPPort)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HealthPort: %w", err)
	}

	cfg.HealthPort = port

	clean, err := boolOr(e.DeleteRow, true)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DeleteRow: %w", err)
	}

	cfg.DeleteRow = clean

	if !clean {
		slog.Warn("delete row is disabled")
	}

	return &cfg, nil
}
