package mariadb_test

import (
	"net"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/richie-tt/mariadb-healthcheck/internal/mariadb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	t.Run("should return error if user is empty", func(t *testing.T) {
		err := (&mariadb.Connection{
			User: "",
		}).Validate()

		require.Error(t, err)
		assert.ErrorContains(t, err, "user is empty")
	})

	t.Run("should return error if password is empty", func(t *testing.T) {
		err := (&mariadb.Connection{
			User:     "user",
			Password: "",
		}).Validate()

		require.Error(t, err)
		assert.ErrorContains(t, err, "password is empty")
	})

	t.Run("should return error if host is empty", func(t *testing.T) {
		err := (&mariadb.Connection{
			User:     "user",
			Password: "password",
			Host:     "",
		}).Validate()

		require.Error(t, err)
		assert.ErrorContains(t, err, "host is empty")
	})

	t.Run("should return error if port is empty", func(t *testing.T) {
		err := (&mariadb.Connection{
			User:     "user",
			Password: "password",
			Host:     "host",
			Port:     "",
		}).Validate()

		require.Error(t, err)
		assert.ErrorContains(t, err, "port is empty")
	})

	t.Run("should return error if database is empty", func(t *testing.T) {
		err := (&mariadb.Connection{
			User:     "user",
			Password: "password",
			Host:     "host",
			Port:     "3306",
			Database: "",
		}).Validate()

		require.Error(t, err)
		assert.ErrorContains(t, err, "database is empty")
	})

	t.Run("should return error if port is not a number", func(t *testing.T) {
		err := (&mariadb.Connection{
			User:     "user",
			Password: "password",
			Host:     "host",
			Port:     "invalid",
			Database: "database",
		}).Validate()

		require.Error(t, err)
		assert.ErrorContains(t, err, "invalid port")
	})

	t.Run("should return error if port is out of range", func(t *testing.T) {
		err := (&mariadb.Connection{
			User:     "user",
			Password: "password",
			Host:     "host",
			Port:     "65536",
			Database: "database",
		}).Validate()

		require.Error(t, err)
		assert.ErrorContains(t, err, "invalid port")
	})
}

func TestConnectDB_DSN_handlesSpecialChars(t *testing.T) {
	conn := mariadb.Connection{ //nolint:gosec // intentional test fixture for special-char password handling
		Driver:   "mysql",
		Database: "healthcheck",
		Host:     "127.0.0.1",
		Password: "p@ss:w/o?rd#",
		Port:     "3306",
		User:     "user",
	}

	db, err := conn.ConnectDB()
	require.NoError(t, err)
	defer db.Close()

	// The pool was configured for sidecar load; verify the cap.
	stats := db.Stats()
	assert.Equal(t, 2, stats.MaxOpenConnections, "MaxOpenConns should be 2")
}

func TestConnectDB_DSN_passwordRoundTrip(t *testing.T) {
	cfg := mysql.NewConfig()
	cfg.User = "user"
	cfg.Passwd = "p@ss:w/o?rd#" //nolint:gosec // intentional test fixture for special-char password handling
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort("127.0.0.1", "3306")
	cfg.DBName = "healthcheck"

	dsn := cfg.FormatDSN()

	parsed, err := mysql.ParseDSN(dsn)
	require.NoError(t, err)
	assert.Equal(t, "p@ss:w/o?rd#", parsed.Passwd)
	assert.Equal(t, "user", parsed.User)
	assert.Equal(t, "127.0.0.1:3306", parsed.Addr)
	assert.Equal(t, "healthcheck", parsed.DBName)
}

func TestConnectDB(t *testing.T) {
	t.Run("should return error if connection is invalid", func(t *testing.T) {
		conn := &mariadb.Connection{}

		_, err := conn.ConnectDB()

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to validate connection")
	})

	t.Run("should return error if connection fails", func(t *testing.T) {
		conn := &mariadb.Connection{
			User:     "user",
			Password: "password",
			Host:     "host",
			Port:     "3306",
			Database: "database",
			Driver:   "unknown",
		}

		_, err := conn.ConnectDB()

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to connect to database")
	})

	t.Run("should run successfully", func(t *testing.T) {
		conn := &mariadb.Connection{
			User:     "user",
			Password: "password",
			Host:     "host",
			Port:     "3306",
			Database: "database",
			Driver:   "mysql",
		}

		_, err := conn.ConnectDB()

		require.NoError(t, err)
	})
}
