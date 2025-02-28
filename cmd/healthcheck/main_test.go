package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupServer(t *testing.T) {
	t.Run("should setup server", func(t *testing.T) {
		server := setupServer(config{
			HealthPort: 8080,
		})

		assert.Equal(t, ":8080", server.Addr)
		assert.NotNil(t, server.Handler)
		assert.Equal(t, httpReadTimeout, server.ReadTimeout)
		assert.Equal(t, httpWriteTimeout, server.WriteTimeout)

		// Test server closes properly
		err := server.Close()
		assert.NoError(t, err, "Server should close without error")
	})

	t.Run("should set up ServeMux", func(t *testing.T) {
		server := setupServer(config{
			HealthPort: 8080,
		})

		// Test if handler is a mux by type assertion
		_, ok := server.Handler.(*http.ServeMux)
		assert.True(t, ok, "Handler should be an http.ServeMux")
	})
}

func TestRun(t *testing.T) {
	t.Run("should return error if env is invalid", func(t *testing.T) {
		t.Setenv("LOG_LEVEL", "invalid")
		err := run()

		assert.Error(t, err)
	})
}
