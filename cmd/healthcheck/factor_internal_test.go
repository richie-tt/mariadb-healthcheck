package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetEnv(t *testing.T) {
	t.Run("should return all env variables", func(t *testing.T) {
		t.Setenv(dbDatabase, "testDB")
		t.Setenv(dbHost, "testHost")
		t.Setenv(dbPassword, "testPassword")
		t.Setenv(dbPort, "testPort")
		t.Setenv(dbUser, "testUser")
		t.Setenv(cleanTable, "true")
		t.Setenv(healthPort, "8080")
		t.Setenv(logLevel, "debug")

		env := getEnv()
		assert.Equal(t, "testDB", env.Connection.Database)
		assert.Equal(t, "testHost", env.Connection.Host)
		assert.Equal(t, "testPassword", env.Connection.Password)
		assert.Equal(t, "testPort", env.Connection.Port)
		assert.Equal(t, "testUser", env.Connection.User)
		assert.Equal(t, "true", env.CleanTable)
		assert.Equal(t, "8080", env.HealthPort)
		assert.Equal(t, "debug", env.LogLevel)
	})
}

func TestParseEnv(t *testing.T) {
	t.Run("should return default values for cleanTable", func(t *testing.T) {
		env := getEnv()
		parsedEnv, err := env.parseEnv()

		require.NoError(t, err)
		assert.True(t, parsedEnv.CleanTable)
	})

	t.Run("should return default values for logLevel", func(t *testing.T) {
		t.Setenv(logLevel, "info")
		env := getEnv()
		parsedEnv, err := env.parseEnv()

		require.NoError(t, err)
		assert.Equal(t, "info", parsedEnv.LogLevel)
	})

	t.Run("should return default values for healthPort", func(t *testing.T) {
		env := getEnv()
		parsedEnv, err := env.parseEnv()

		require.NoError(t, err)
		assert.Equal(t, 8080, parsedEnv.HealthPort)
	})

	t.Run("should return error for invalid healthPort", func(t *testing.T) {
		env := getEnv()
		env.HealthPort = "invalid"
		_, err := env.parseEnv()

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to parse HealthPort")
	})

	t.Run("should return error for invalid cleanTable", func(t *testing.T) {
		env := getEnv()
		env.CleanTable = "invalid"
		_, err := env.parseEnv()

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to parse CleanTable")
	})
}
