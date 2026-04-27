package mariadb_test

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/richie-tt/mariadb-healthcheck/internal/mariadb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCheck(t *testing.T) {
	const uuid = "test-id"

	t.Run("should succeed when delete is enabled", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(uuid).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(uuid).
			WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow(uuid))
		mock.ExpectExec("DELETE FROM status WHERE uuid = ?").
			WithArgs(uuid).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err = mariadb.RunCheck(t.Context(), db, uuid, true)

		assert.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("should succeed when delete is disabled", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(uuid).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(uuid).
			WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow(uuid))

		err = mariadb.RunCheck(t.Context(), db, uuid, false)

		assert.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("should return ErrInsert on insert failure", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(uuid).
			WillReturnError(errors.New("insert failed"))

		err = mariadb.RunCheck(t.Context(), db, uuid, true)

		require.Error(t, err)
		require.ErrorIs(t, err, mariadb.ErrInsert)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("should return ErrSelect on select failure", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(uuid).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(uuid).
			WillReturnError(errors.New("select failed"))

		err = mariadb.RunCheck(t.Context(), db, uuid, true)

		require.Error(t, err)
		require.ErrorIs(t, err, mariadb.ErrSelect)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("should return ErrValidate when value mismatches", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(uuid).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(uuid).
			WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow("different"))

		err = mariadb.RunCheck(t.Context(), db, uuid, true)

		require.Error(t, err)
		require.ErrorIs(t, err, mariadb.ErrValidate)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("should return ErrDelete on delete failure", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(uuid).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(uuid).
			WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow(uuid))
		mock.ExpectExec("DELETE FROM status WHERE uuid = ?").
			WithArgs(uuid).
			WillReturnError(errors.New("delete failed"))

		err = mariadb.RunCheck(t.Context(), db, uuid, true)

		require.Error(t, err)
		require.ErrorIs(t, err, mariadb.ErrDelete)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
