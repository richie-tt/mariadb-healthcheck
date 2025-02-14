package mariadb

import (
	"context"
	"database/sql"
	"fmt"
)

// InsertRow inserts a row into the database
func (q Query) InsertRow(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, "INSERT INTO status (uuid) VALUES (?)", q.Value)
	if err != nil {
		return fmt.Errorf("failed to insert: %w", err)
	}

	return nil
}

// SelectRow selects a row from the database
func (q Query) SelectRow(ctx context.Context, db *sql.DB) (*sql.Row, error) {
	row := db.QueryRowContext(ctx, "SELECT uuid FROM status WHERE uuid = ?", q.Value)
	if row.Err() != nil {
		return nil, fmt.Errorf("failed to select: %w", row.Err())
	}

	return row, nil
}

// DeleteRow deletes a row from the database
func (q Query) DeleteRow(ctx context.Context, db *sql.DB) error {
	// func (q Query) DeleteRow(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, "DELETE FROM status WHERE uuid = ?", q.Value)
	if err != nil {
		return fmt.Errorf("failed to delete: %w", err)
	}

	return nil
}
