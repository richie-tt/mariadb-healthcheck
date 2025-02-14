package main

import (
	"context"
	"log/slog"
	"mariadb"
	"net/http"

	"github.com/google/uuid"
)

func (c config) healthHandler(w http.ResponseWriter, _ *http.Request) {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
		slog.Debug("generated id", "id", c.ID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	query := mariadb.Query{
		Value: c.ID.String(),
	}

	if err := query.InsertRow(ctx, c.DBInterface); err != nil {
		slog.ErrorContext(
			ctx,
			"failed to insert row",
			"error", err,
		)

		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("failed to insert row"))
		return
	}

	slog.Debug("inserted row", "id", c.ID)

	row, err := query.SelectRow(ctx, c.DBInterface)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"failed to select row",
			"error", err,
		)

		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("failed to select row"))
		return
	}

	slog.Debug("selected row", "id", c.ID)

	var value string
	if err := row.Scan(&value); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("failed to scan row"))
		return
	}

	if value != c.ID.String() {
		slog.Error(
			"Value is not the same",
			"expected", c.ID.String(),
			"got", value,
		)

		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("failed to validate row"))
		return
	}

	if c.CleanTable {
		if err := query.DeleteRow(ctx, c.DBInterface); err != nil {
			slog.ErrorContext(
				ctx,
				"failed to delete row",
				"error", err,
			)

			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("failed to delete row"))
			return
		}

		slog.Debug("deleted row", "id", c.ID)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
