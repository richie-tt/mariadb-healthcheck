package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	env := getEnv()

	config, err := env.parseEnv()
	if err != nil {
		slog.Error(
			"failed to parse environment",
			"error", err,
		)
		os.Exit(1)
	}

	db, err := config.Connection.ConnectDB()
	if err != nil {
		slog.Error(
			"failed to connect to database",
			"error", err,
		)
		os.Exit(1)
	}

	defer db.Close()

	config.DBInterface = db

	mux := http.NewServeMux()
	mux.HandleFunc("/health", config.healthHandler)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", config.HealthPort),
		Handler:      mux,
		ReadTimeout:  httpReadTimeout,
		WriteTimeout: httpWriteTimeout,
	}

	slog.Info(
		"starting health check server",
		"port", config.HealthPort,
	)

	if err := server.ListenAndServe(); err != nil {
		slog.Error(
			"failed to start server",
			"error", err,
		)
		db.Close() // Explicitly close the DB connection before exiting
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
