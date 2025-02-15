package main

import "time"

const (
	dbDatabase = "DB_DATABASE"
	dbUser     = "DB_USER"
	dbPassword = "DB_PASSWORD"
	dbHost     = "DB_HOST"
	dbPort     = "DB_PORT"
	logLevel   = "LOG_LEVEL"
	cleanTable = "CLEAN_TABLE"
	healthPort = "HEALTH_PORT"

	contextTimeout   = time.Second * 5
	httpReadTimeout  = time.Second * 5
	httpWriteTimeout = time.Second * 5
)
