package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetEnv(t *testing.T) {
	t.Run("should return all env variables", func(t *testing.T) {
		t.Setenv(dbName, "testDB")
		t.Setenv(dbHost, "testHost")
		t.Setenv(dbPassword, "testPassword")
		t.Setenv(dbPort, "testPort")
		t.Setenv(dbUser, "testUser")
		t.Setenv(deleteRow, "true")
		t.Setenv(healthPort, "8080")
		t.Setenv(logLevel, "debug")

		env := getEnv()
		assert.Equal(t, "testDB", env.Connection.Database)
		assert.Equal(t, "testHost", env.Connection.Host)
		assert.Equal(t, "testPassword", env.Connection.Password)
		assert.Equal(t, "testPort", env.Connection.Port)
		assert.Equal(t, "testUser", env.Connection.User)
		assert.Equal(t, "true", env.DeleteRow)
		assert.Equal(t, "8080", env.HealthPort)
		assert.Equal(t, "debug", env.LogLevel)
	})
}

func TestParseEnv(t *testing.T) {
	t.Run("should return default values for database", func(t *testing.T) {
		t.Setenv(dbName, "")
		parsedEnv, err := getEnv().parseEnv()

		require.NoError(t, err)
		assert.Equal(t, "healthcheck", parsedEnv.Connection.Database)
	})

	t.Run("should return default values for host", func(t *testing.T) {
		t.Setenv(dbHost, "")
		parsedEnv, err := getEnv().parseEnv()

		require.NoError(t, err)
		assert.Equal(t, "127.0.0.1", parsedEnv.Connection.Host)
	})

	t.Run("should return default values for port", func(t *testing.T) {
		t.Setenv(dbPort, "")
		parsedEnv, err := getEnv().parseEnv()

		require.NoError(t, err)
		assert.Equal(t, "3306", parsedEnv.Connection.Port)
	})

	t.Run("should return default values for user", func(t *testing.T) {
		t.Setenv(dbUser, "")
		parsedEnv, err := getEnv().parseEnv()

		require.NoError(t, err)
		assert.Equal(t, "healthcheck", parsedEnv.Connection.User)
	})

	t.Run("should return default values for password", func(t *testing.T) {
		t.Setenv(dbPassword, "")
		parsedEnv, err := getEnv().parseEnv()

		require.NoError(t, err)
		assert.Equal(t, "healthcheck", parsedEnv.Connection.Password)
	})

	t.Run("should return default values for logLevel", func(t *testing.T) {
		t.Setenv(logLevel, "")
		parsedEnv, err := getEnv().parseEnv()

		require.NoError(t, err)
		assert.Equal(t, "info", parsedEnv.LogLevel)
	})

	t.Run("should return error for invalid logLevel", func(t *testing.T) {
		t.Setenv(logLevel, "invalid")

		_, err := getEnv().parseEnv()

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to parse the log level")
	})

	t.Run("should return default values for healthPort", func(t *testing.T) {
		parsedEnv, err := getEnv().parseEnv()

		require.NoError(t, err)
		assert.Equal(t, 8080, parsedEnv.HealthPort)
	})

	t.Run("should return error for invalid healthPort", func(t *testing.T) {
		t.Setenv(healthPort, "invalid")
		_, err := getEnv().parseEnv()

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to parse HealthPort")
	})

	t.Run("should return parsed custom values for healthPort", func(t *testing.T) {
		t.Setenv(healthPort, "8081")
		parsedEnv, err := getEnv().parseEnv()

		require.NoError(t, err)
		assert.Equal(t, 8081, parsedEnv.HealthPort)
	})

	t.Run("should return default values for cleanTable", func(t *testing.T) {
		parsedEnv, err := getEnv().parseEnv()

		require.NoError(t, err)
		assert.True(t, parsedEnv.DeleteRow)
	})

	t.Run("should return error for invalid deleteRow", func(t *testing.T) {
		t.Setenv(deleteRow, "invalid")
		_, err := getEnv().parseEnv()

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to parse DeleteRow")
	})

	t.Run("should return parsed custom values for deleteRow", func(t *testing.T) {
		t.Setenv(deleteRow, "false")
		parsedEnv, err := getEnv().parseEnv()

		require.NoError(t, err)
		assert.False(t, parsedEnv.DeleteRow)
	})
}
