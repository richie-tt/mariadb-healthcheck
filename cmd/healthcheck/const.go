package main

import "time"

const (
	dbName     = "DB_NAME"
	dbUser     = "DB_USER"
	dbPassword = "DB_PASSWORD"
	dbHost     = "DB_HOST"
	dbPort     = "DB_PORT"
	logLevel   = "LOG_LEVEL"
	deleteRow  = "DELETE_ROW"
	healthPort = "HEALTH_PORT"

	contextTimeout   = time.Second * 5
	httpReadTimeout  = time.Second * 5
	httpWriteTimeout = time.Second * 5

	defaultDBUser     = "healthcheck"
	defaultDBPassword = "healthcheck"
	defaultDBHost     = "127.0.0.1"
	defaultDBPort     = "3306"
	defaultDBName     = "healthcheck"
)
