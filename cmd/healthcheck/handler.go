package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/richie-tt/mariadb-healthcheck/internal/mariadb"
)

func (c config) healthHandler(w http.ResponseWriter, r *http.Request) {
	id := uuid.New()

	slog.Debug(
		"generated UUID",
		"value", id,
	)

	ctx, cancel := context.WithTimeout(r.Context(), contextTimeout)
	defer cancel()

	err := mariadb.RunCheck(ctx, c.DBInterface, id.String(), c.DeleteRow)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	if err == nil {
		w.WriteHeader(http.StatusOK)
		writeBody(w, "OK")
		return
	}

	slog.ErrorContext(ctx, "healthcheck failed", "error", err)

	w.WriteHeader(http.StatusInternalServerError)

	switch {
	case errors.Is(err, mariadb.ErrInsert):
		writeBody(w, "failed to insert row")
	case errors.Is(err, mariadb.ErrSelect):
		writeBody(w, "failed to select row")
	case errors.Is(err, mariadb.ErrScan):
		writeBody(w, "failed to scan row")
	case errors.Is(err, mariadb.ErrValidate):
		writeBody(w, "failed to validate row")
	case errors.Is(err, mariadb.ErrDelete):
		writeBody(w, "failed to delete row")
	default:
		writeBody(w, "healthcheck failed")
	}
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
