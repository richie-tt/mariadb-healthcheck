package main

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		assert.Equal(t, httpReadHeaderTimeout, server.ReadHeaderTimeout)
		assert.Equal(t, httpIdleTimeout, server.IdleTimeout)

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

func TestRun_gracefulShutdown(t *testing.T) {
	// We can't easily exercise the full run() path in a unit test (it dials
	// a real DB). Verify instead that an http.Server returned by setupServer
	// supports Shutdown without error.
	srv := setupServer(config{HealthPort: 0})

	listenErr := make(chan error, 1)
	go func() {
		listenErr <- srv.ListenAndServe()
	}()

	// Give the listener a moment to bind.
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.NoError(t, srv.Shutdown(ctx))
	require.ErrorIs(t, <-listenErr, http.ErrServerClosed)
}

func TestAwaitShutdown(t *testing.T) {
	t.Run("shuts server down once context is canceled", func(t *testing.T) {
		srv := setupServer(config{HealthPort: 0})

		listenErr := make(chan error, 1)
		go func() {
			listenErr <- srv.ListenAndServe()
		}()

		// Let the listener bind before we start the shutdown waiter.
		time.Sleep(50 * time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			awaitShutdown(ctx, srv)
			close(done)
		}()

		cancel()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("awaitShutdown did not return after ctx cancel")
		}

		require.ErrorIs(t, <-listenErr, http.ErrServerClosed)
	})

	t.Run("returns even when server has already stopped", func(t *testing.T) {
		srv := setupServer(config{HealthPort: 0})
		require.NoError(t, srv.Close())

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		done := make(chan struct{})
		go func() {
			awaitShutdown(ctx, srv)
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("awaitShutdown did not return when server was already closed")
		}
	})
}
