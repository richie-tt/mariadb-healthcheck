// Package mariadb provides a connection to a MariaDB database.
package mariadb

import (
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
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

	_, err := url.Parse(c.Host)
	if err != nil {
		return fmt.Errorf("invalid host: %w", err)
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

// ConnectDB connects to the database
func (c Connection) ConnectDB() (*sql.DB, error) {
	err := c.Validate()
	if err != nil {
		return nil, fmt.Errorf("failed to validate connection: %w", err)
	}

	connectionString := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", c.User, c.Password, c.Host, c.Port, c.Database)
	db, err := sql.Open(c.Driver, connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}
