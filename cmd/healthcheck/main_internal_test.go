package main

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLogLevel(t *testing.T) {
	t.Run("should return debug level", func(t *testing.T) {
		config := config{LogLevel: "debug"}
		level, err := config.getLogLevel()

		require.NoError(t, err)
		assert.Equal(t, slog.LevelDebug, level)
	})

	t.Run("should return info level", func(t *testing.T) {
		config := config{LogLevel: "info"}
		level, err := config.getLogLevel()

		require.NoError(t, err)
		assert.Equal(t, slog.LevelInfo, level)
	})

	t.Run("should return warn level", func(t *testing.T) {
		config := config{LogLevel: "warn"}
		level, err := config.getLogLevel()

		require.NoError(t, err)
		assert.Equal(t, slog.LevelWarn, level)
	})

	t.Run("should return error level", func(t *testing.T) {
		config := config{LogLevel: "error"}
		level, err := config.getLogLevel()

		require.NoError(t, err)
		assert.Equal(t, slog.LevelError, level)
	})

	t.Run("should return info level for invalid log level", func(t *testing.T) {
		config := config{LogLevel: "invalid"}
		level, err := config.getLogLevel()

		require.Error(t, err)
		assert.Equal(t, slog.LevelInfo, level)
	})
}
