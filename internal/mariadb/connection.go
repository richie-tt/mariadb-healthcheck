// Package mariadb provides a connection to a MariaDB database.
package mariadb

import (
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
)

const (
	dbConnectTimeout  = 5 * time.Second
	dbMaxOpenConns    = 2
	dbMaxIdleConns    = 1
	dbConnMaxLifetime = 5 * time.Minute
	dbConnMaxIdleTime = 1 * time.Minute
)

// Validate validates the connection
func (c *Connection) Validate() error {
	if c.User == "" {
		return fmt.Errorf("user is empty")
	}

	if c.Password == "" {
		return fmt.Errorf("password is empty")
	}

	if c.Host == "" {
		return fmt.Errorf("host is empty")
	}

	if c.Port == "" {
		return fmt.Errorf("port is empty")
	}

	if c.Database == "" {
		return fmt.Errorf("database is empty")
	}

	port, err := strconv.Atoi(c.Port)
	if err != nil {
		return fmt.Errorf("invalid port: %w", err)
	}

	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}

	return nil
}

// ConnectDB connects to the database using a DSN built via mysql.Config so
// that special characters in the password are escaped correctly.
func (c Connection) ConnectDB() (*sql.DB, error) {
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate connection: %w", err)
	}

	cfg := mysql.NewConfig()
	cfg.User = c.User
	cfg.Passwd = c.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(c.Host, c.Port)
	cfg.DBName = c.Database
	cfg.ParseTime = true
	cfg.Timeout = dbConnectTimeout

	db, err := sql.Open(c.Driver, cfg.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db.SetMaxOpenConns(dbMaxOpenConns)
	db.SetMaxIdleConns(dbMaxIdleConns)
	db.SetConnMaxLifetime(dbConnMaxLifetime)
	db.SetConnMaxIdleTime(dbConnMaxIdleTime)

	return db, nil
}
