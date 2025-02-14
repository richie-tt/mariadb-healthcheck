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

		slog.Debug(
			"generated UUID",
			"value", c.ID,
		)
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
		writeBody(w, "failed to insert row")
		return
	}

	slog.Debug(
		"Executed query to insert row",
		"UUID", c.ID,
	)

	row, err := query.SelectRow(ctx, c.DBInterface)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"failed to select row",
			"error", err,
		)

		w.WriteHeader(http.StatusInternalServerError)
		writeBody(w, "failed to select row")
		return
	}

	slog.Debug(
		"Executed query to select row",
		"UUID", c.ID,
	)

	var value string
	if err := row.Scan(&value); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeBody(w, "failed to scan row")
		return
	}

	if value != c.ID.String() {
		slog.Error(
			"Value is not the same",
			"expected", c.ID.String(),
			"got", value,
		)

		w.WriteHeader(http.StatusInternalServerError)
		writeBody(w, "failed to validate row")
		return
	}

	if c.DeleteRow {
		if err := query.DeleteRow(ctx, c.DBInterface); err != nil {
			slog.ErrorContext(
				ctx,
				"failed to delete row",
				"error", err,
			)

			w.WriteHeader(http.StatusInternalServerError)
			writeBody(w, "failed to delete row")
			return
		}

		slog.Debug(
			"Executed query to delete row",
			"UUID", c.ID,
		)
	}

	w.WriteHeader(http.StatusOK)
	writeBody(w, "OK")
}

func writeBody(w http.ResponseWriter, message string) {
	_, err := w.Write([]byte(message))
	if err != nil {
		slog.Error(
			"failed to write body",
			"message", message,
			"error", err,
		)
	}
}
