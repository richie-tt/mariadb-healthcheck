package mariadb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
)

// Sentinel errors for the health-check stages. Consumers should match on
// these with errors.Is to map a check failure to a user-facing message.
var (
	ErrInsert   = errors.New("failed to insert row")
	ErrSelect   = errors.New("failed to select row")
	ErrScan     = errors.New("failed to scan row")
	ErrValidate = errors.New("failed to validate row")
	ErrDelete   = errors.New("failed to delete row")
)

// RunCheck executes the INSERT -> SELECT -> (optional) DELETE health-check
// sequence using uuid as the UUID-shaped value written to the status table.
// On failure it returns one of the sentinel errors above wrapped with the
// underlying cause. Stage errors are NOT logged here — the HTTP handler is
// the single error-logging boundary so callers can adjust verbosity in one
// place.
func RunCheck(ctx context.Context, db *sql.DB, uuid string, deleteRow bool) error {
	if err := InsertRow(ctx, db, uuid); err != nil {
		return fmt.Errorf("%w: %v", ErrInsert, err)
	}

	slog.Debug( //nolint:G706 // UUID has fixed format
		"Executed query to insert row",
		"UUID", uuid,
	)

	row, err := SelectRow(ctx, db, uuid)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSelect, err)
	}

	slog.Debug( //nolint:G706 // UUID has fixed format
		"Executed query to select row",
		"UUID", uuid,
	)

	var value string
	if err := row.Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: inserted row not found", ErrValidate)
		}

		return fmt.Errorf("%w: %v", ErrScan, err)
	}

	_ = value

	if deleteRow {
		if err := DeleteRow(ctx, db, uuid); err != nil {
			return fmt.Errorf("%w: %v", ErrDelete, err)
		}

		slog.Debug( //nolint:G706 // UUID has fixed format
			"Executed query to delete row",
			"UUID", uuid,
		)
	}

	return nil
}
