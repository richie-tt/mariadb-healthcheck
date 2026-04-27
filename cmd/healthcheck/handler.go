package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/richie-tt/mariadb-healthcheck/internal/mariadb"
)

func (c config) healthHandler(w http.ResponseWriter, _ *http.Request) {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()

		slog.Debug( //nolint:G706 // UUID has fixed format
			"generated UUID",
			"value", c.ID,
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	err := mariadb.RunCheck(ctx, c.DBInterface, c.ID.String(), c.DeleteRow)
	if err == nil {
		w.WriteHeader(http.StatusOK)
		writeBody(w, "OK")
		return
	}

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
