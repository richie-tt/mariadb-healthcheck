package main

import (
	"fmt"
	"log/slog"
	"mariadb"
	"os"
	"strconv"
)

func getEnv() environment {
	return environment{
		Connection: mariadb.Connection{
			Database: os.Getenv(dbDatabase),
			Driver:   "mysql",
			Host:     os.Getenv(dbHost),
			Password: os.Getenv(dbPassword),
			Port:     os.Getenv(dbPort),
			User:     os.Getenv(dbUser),
		},
		CleanTable: os.Getenv(cleanTable),
		HealthPort: os.Getenv(healthPort),
		LogLevel:   os.Getenv(logLevel),
	}
}

func (e environment) parseEnv() (*config, error) {
	config := config{
		Connection: e.Connection,
		LogLevel:   e.LogLevel,
	}

	if config.Connection.Database == "" {
		config.Connection.Database = defaultDBDatabase
	}

	if config.Connection.Host == "" {
		config.Connection.Host = defaultDBHost
	}

	if config.Connection.Port == "" {
		config.Connection.Port = defaultDBPort
	}

	if config.Connection.User == "" {
		config.Connection.User = defaultDBUser
	}

	if config.Connection.Password == "" {
		config.Connection.Password = defaultDBPassword
	}

	if e.LogLevel == "" {
		config.LogLevel = "info"
		slog.Info("using default log level", "logLevel", config.LogLevel)
	}

	logLevel, err := config.getLogLevel()
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
					Level: logLevel,
				},
			),
		),
	)

	if e.CleanTable == "" {
		config.CleanTable = true
		slog.Debug("using default clean table", "cleanTable", config.CleanTable)
	}

	if e.HealthPort == "" {
		config.HealthPort = 8080
		slog.Debug("using default health port", "healthPort", config.HealthPort)
	}

	if e.HealthPort != "" {
		healthPort, err := strconv.Atoi(e.HealthPort)
		if err != nil {
			return nil, fmt.Errorf("failed to parse HealthPort: %w", err)
		}

		config.HealthPort = healthPort
	}

	if e.CleanTable != "" {
		cleanTable, err := strconv.ParseBool(e.CleanTable)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CleanTable: %w", err)
		}

		if !cleanTable {
			slog.Warn("clean table is disabled")
		}

		config.CleanTable = cleanTable
		slog.Debug("parsed clean table", "cleanTable", config.CleanTable)
	}

	return &config, nil
}
