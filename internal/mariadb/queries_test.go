package mariadb_test

import (
	"errors"
	"mariadb"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsertRow(t *testing.T) {
	t.Run("should return error if insert fails", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("failed to create mock database: %v", err)
		}
		defer db.Close()

		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs("1").
			WillReturnError(errors.New("insert failed"))

		err = mariadb.Query{
			Value: "1",
		}.InsertRow(t.Context(), db)

		require.NoError(t, mock.ExpectationsWereMet())
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to insert")
	})

	t.Run("should insert row successfully", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("failed to create mock database: %v", err)
		}
		defer db.Close()

		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs("1").
			WillReturnResult(sqlmock.NewResult(1, 1))

		err = mariadb.Query{
			Value: "1",
		}.InsertRow(t.Context(), db)

		require.NoError(t, mock.ExpectationsWereMet())
		assert.NoError(t, err)
	})
}

func TestSelectRow(t *testing.T) {
	t.Run("should select row successfully", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("failed to create mock database: %v", err)
		}
		defer db.Close()

		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs("1").
			WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow("1"))

		row, err := mariadb.Query{
			Value: "1",
		}.SelectRow(t.Context(), db)

		require.NoError(t, mock.ExpectationsWereMet())
		assert.NoError(t, err)
		assert.NotNil(t, row)
	})

	t.Run("should return error if select fails", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("failed to create mock database: %v", err)
		}
		defer db.Close()

		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs("1").
			WillReturnError(errors.New("select failed"))

		_, err = mariadb.Query{
			Value: "1",
		}.SelectRow(t.Context(), db)

		require.NoError(t, mock.ExpectationsWereMet())
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to select")
	})
}

func TestDeleteRow(t *testing.T) {
	t.Run("should delete row successfully", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("failed to create mock database: %v", err)
		}
		defer db.Close()

		mock.ExpectExec("DELETE FROM status WHERE uuid = ?").
			WithArgs("1").
			WillReturnResult(sqlmock.NewResult(1, 1))

		err = mariadb.Query{
			Value: "1",
		}.DeleteRow(t.Context(), db)

		require.NoError(t, mock.ExpectationsWereMet())
		assert.NoError(t, err)
	})

	t.Run("should return error if delete fails", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("failed to create mock database: %v", err)
		}
		defer db.Close()

		mock.ExpectExec("DELETE FROM status WHERE uuid = ?").
			WithArgs("1").
			WillReturnError(errors.New("delete failed"))

		err = mariadb.Query{
			Value: "1",
		}.DeleteRow(t.Context(), db)

		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
		assert.ErrorContains(t, err, "failed to delete")
	})
}
