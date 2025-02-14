package main

import (
	"database/sql"
	"mariadb"

	"github.com/google/uuid"
)

type environment struct {
	DeleteRow  string
	Connection mariadb.Connection
	HealthPort string
	LogLevel   string
}

type config struct {
	Connection  mariadb.Connection
	DBInterface *sql.DB
	DeleteRow   bool
	HealthPort  int
	ID          uuid.UUID
	LogLevel    string
}
