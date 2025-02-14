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
	config := config{
		Connection: e.Connection,
		LogLevel:   e.LogLevel,
	}

	if config.Connection.Database == "" {
		config.Connection.Database = defaultDBName
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
		slog.Info(
			"using default log level",
			"value", config.LogLevel,
		)
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

	if e.DeleteRow == "" {
		config.DeleteRow = true
		slog.Debug(
			"using default delete row",
			"value", config.DeleteRow,
		)
	}

	if e.HealthPort == "" {
		config.HealthPort = 8080
		slog.Debug(
			"using default health port",
			"value", config.HealthPort,
		)
	}

	if e.HealthPort != "" {
		healthPort, err := strconv.Atoi(e.HealthPort)
		if err != nil {
			return nil, fmt.Errorf("failed to parse HealthPort: %w", err)
		}

		config.HealthPort = healthPort
	}

	if e.DeleteRow != "" {
		deleteRow, err := strconv.ParseBool(e.DeleteRow)
		if err != nil {
			return nil, fmt.Errorf("failed to parse DeleteRow: %w", err)
		}

		if !deleteRow {
			slog.Warn("delete row is disabled")
		}

		config.DeleteRow = deleteRow
		slog.Debug(
			"parsed delete row",
			"value", config.DeleteRow,
		)
	}

	return &config, nil
}
