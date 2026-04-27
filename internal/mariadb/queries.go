package mariadb

import (
	"context"
	"database/sql"
	"fmt"
)

// InsertRow inserts a row into the status table for the given value.
func InsertRow(ctx context.Context, db *sql.DB, value string) error {
	_, err := db.ExecContext(ctx, "INSERT INTO status (uuid) VALUES (?)", value)
	if err != nil {
		return fmt.Errorf("InsertRow: %w", err)
	}

	return nil
}

// SelectRow selects a row from the status table matching the given value.
func SelectRow(ctx context.Context, db *sql.DB, value string) (*sql.Row, error) {
	row := db.QueryRowContext(ctx, "SELECT uuid FROM status WHERE uuid = ?", value)
	if row.Err() != nil {
		return nil, fmt.Errorf("SelectRow: %w", row.Err())
	}

	return row, nil
}

// DeleteRow deletes a row from the status table matching the given value.
func DeleteRow(ctx context.Context, db *sql.DB, value string) error {
	_, err := db.ExecContext(ctx, "DELETE FROM status WHERE uuid = ?", value)
	if err != nil {
		return fmt.Errorf("DeleteRow: %w", err)
	}

	return nil
}
