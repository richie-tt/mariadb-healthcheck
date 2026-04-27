package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
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
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("failed to create mock database: %v", err)
		}

		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(sqlmock.AnyArg()).
			WillReturnError(errors.New("insert failed"))

		defer db.Close()

		server := httptest.NewServer(
			http.HandlerFunc(
				config{
					DBInterface: db,
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
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("failed to create mock database: %v", err)
		}

		defer db.Close()
		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(sqlmock.AnyArg()).
			WillReturnError(errors.New("select failed"))

		server := httptest.NewServer(
			http.HandlerFunc(
				config{
					DBInterface: db,
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

	t.Run("should return failed to scan row when scan errors", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("failed to create mock database: %v", err)
		}

		defer db.Close()
		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// A row exists but scanning it fails — this is the ErrScan path,
		// distinct from the sql.ErrNoRows path that maps to ErrValidate.
		rows := sqlmock.NewRows([]string{"uuid"}).
			AddRow("any-uuid").
			RowError(0, errors.New("scan failed"))

		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(sqlmock.AnyArg()).
			WillReturnRows(rows)

		server := httptest.NewServer(
			http.HandlerFunc(
				config{
					DBInterface: db,
				}.healthHandler,
			),
		)
		defer server.Close()

		resp, err := http.Get(server.URL)
		body := decodeHTTPBody(t, resp)

		require.NoError(t, mock.ExpectationsWereMet())
		require.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		assert.Equal(t, "failed to scan row", body)
	})

	t.Run("should return failed to validate row when SELECT returns no rows", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("failed to create mock database: %v", err)
		}

		defer db.Close()
		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"uuid"}))

		server := httptest.NewServer(
			http.HandlerFunc(
				config{
					DBInterface: db,
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
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("failed to create mock database: %v", err)
		}

		defer db.Close()
		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow("any-uuid"))

		server := httptest.NewServer(
			http.HandlerFunc(
				config{
					DBInterface: db,
					DeleteRow:   false,
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
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("failed to create mock database: %v", err)
		}

		defer db.Close()
		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow("any-uuid"))

		mock.ExpectExec("DELETE FROM status WHERE uuid = ?").
			WithArgs(sqlmock.AnyArg()).
			WillReturnError(errors.New("delete failed"))

		server := httptest.NewServer(
			http.HandlerFunc(
				config{
					DBInterface: db,
					DeleteRow:   true,
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
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("failed to create mock database: %v", err)
		}

		defer db.Close()
		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow("any-uuid"))

		mock.ExpectExec("DELETE FROM status WHERE uuid = ?").
			WithArgs(sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		server := httptest.NewServer(
			http.HandlerFunc(
				config{
					DBInterface: db,
					DeleteRow:   true,
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
		assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
	})
}

func TestWriteBody(t *testing.T) {
	t.Run("should write message to response body", func(t *testing.T) {
		w := httptest.NewRecorder()
		writeBody(w, "test response")

		assert.Equal(t, "test response", w.Body.String())
	})

	t.Run("should trigger error when writing body", func(_ *testing.T) {
		// function used to handle coverage
		w := &errorWriter{httptest.NewRecorder()}

		writeBody(w, "test response")
	})
}

type errorWriter struct {
	http.ResponseWriter
}

func (e *errorWriter) Write([]byte) (int, error) {
	return 0, errors.New("forced write error")
}
