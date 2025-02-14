package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func decodeHTTPBody(t *testing.T, resp *http.Response) string {
	t.Helper()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	defer resp.Body.Close()

	return string(body)
}

func TestHealthHandler(t *testing.T) {
	t.Run("should return failed to insert row", func(t *testing.T) {
		uid := uuid.New()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("failed to create mock database: %v", err)
		}

		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(uid.String()).
			WillReturnError(errors.New("insert failed"))

		defer db.Close()

		server := httptest.NewServer(
			http.HandlerFunc(
				config{
					DBInterface: db,
					ID:          uid,
				}.healthHandler,
			),
		)
		defer server.Close()

		resp, err := http.Get(server.URL)
		body := decodeHTTPBody(t, resp)

		require.NoError(t, mock.ExpectationsWereMet())
		require.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		assert.Equal(t, "failed to insert row", body)
	})

	t.Run("should return failed to select row", func(t *testing.T) {
		uid := uuid.New()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("failed to create mock database: %v", err)
		}

		defer db.Close()
		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(uid.String()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(uid.String()).
			WillReturnError(errors.New("select failed"))

		server := httptest.NewServer(
			http.HandlerFunc(
				config{
					DBInterface: db,
					ID:          uid,
				}.healthHandler,
			),
		)
		defer server.Close()

		resp, err := http.Get(server.URL)
		body := decodeHTTPBody(t, resp)

		require.NoError(t, mock.ExpectationsWereMet())
		require.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		assert.Equal(t, "failed to select row", body)
	})

	t.Run("should return failed to validate row", func(t *testing.T) {
		uid := uuid.New()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("failed to create mock database: %v", err)
		}

		defer db.Close()
		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(uid.String()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(uid.String()).
			WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow("1"))

		server := httptest.NewServer(
			http.HandlerFunc(
				config{
					DBInterface: db,
					ID:          uid,
				}.healthHandler,
			),
		)
		defer server.Close()

		resp, err := http.Get(server.URL)
		body := decodeHTTPBody(t, resp)

		require.NoError(t, mock.ExpectationsWereMet())
		require.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		assert.Equal(t, "failed to validate row", body)
	})

	t.Run("should return OK, when clean table is false", func(t *testing.T) {
		uid := uuid.New()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("failed to create mock database: %v", err)
		}

		defer db.Close()
		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(uid.String()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(uid.String()).
			WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow(uid.String()))

		server := httptest.NewServer(
			http.HandlerFunc(
				config{
					DBInterface: db,
					ID:          uid,
					CleanTable:  false,
				}.healthHandler,
			),
		)
		defer server.Close()

		resp, err := http.Get(server.URL)
		body := decodeHTTPBody(t, resp)

		require.NoError(t, mock.ExpectationsWereMet())
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "OK", body)
	})

	t.Run("should return failed to delete row", func(t *testing.T) {
		uid := uuid.New()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("failed to create mock database: %v", err)
		}

		defer db.Close()
		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(uid.String()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(uid.String()).
			WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow(uid.String()))

		mock.ExpectExec("DELETE FROM status WHERE uuid = ?").
			WithArgs(uid.String()).
			WillReturnError(errors.New("delete failed"))

		server := httptest.NewServer(
			http.HandlerFunc(
				config{
					DBInterface: db,
					ID:          uid,
					CleanTable:  true,
				}.healthHandler,
			),
		)
		defer server.Close()

		resp, err := http.Get(server.URL)
		body := decodeHTTPBody(t, resp)

		require.NoError(t, mock.ExpectationsWereMet())
		require.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		assert.Equal(t, "failed to delete row", body)
	})

	t.Run("should return OK, when clean table is true", func(t *testing.T) {
		uid := uuid.New()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("failed to create mock database: %v", err)
		}

		defer db.Close()
		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(uid.String()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(uid.String()).
			WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow(uid.String()))

		mock.ExpectExec("DELETE FROM status WHERE uuid = ?").
			WithArgs(uid.String()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		server := httptest.NewServer(
			http.HandlerFunc(
				config{
					DBInterface: db,
					ID:          uid,
					CleanTable:  true,
				}.healthHandler,
			),
		)
		defer server.Close()

		resp, err := http.Get(server.URL)
		body := decodeHTTPBody(t, resp)

		require.NoError(t, mock.ExpectationsWereMet())
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "OK", body)
	})
}
