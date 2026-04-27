# PR 1: Refactor — Single Go Module + Handler Split + Drop Query Type

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure the codebase without changing runtime behavior: collapse the dual `go.mod` / `go.work` setup into a single root Go module, replace the `Query` struct's methods with plain package-level functions, and extract the INSERT → SELECT → DELETE sequence from the HTTP handler into a `RunCheck` function in the `mariadb` package. All HTTP status codes, body strings, and observable log content stay identical.

**Architecture:** Single `go.mod` at repo root with module path `github.com/richie-tt/mariadb-healthcheck`. SQL operations live in `internal/mariadb` as package-level functions: `InsertRow(ctx, db, value)`, `SelectRow(ctx, db, value)`, `DeleteRow(ctx, db, value)`. Domain orchestration lives in `RunCheck(ctx, db, id, deleteRow)` which returns sentinel errors (`ErrInsert`, `ErrSelect`, `ErrScan`, `ErrValidate`, `ErrDelete`). The HTTP handler in `cmd/healthcheck/handler.go` becomes a thin marshaling layer: generate UUID → call `RunCheck` → map sentinel error to body string and status code.

**Tech Stack:** Go 1.25, `database/sql`, `github.com/go-sql-driver/mysql` v1.9.3, `github.com/google/uuid` v1.6.0, `github.com/DATA-DOG/go-sqlmock` v1.5.2, `github.com/stretchr/testify` v1.11.1.

**Spec:** `docs/superpowers/specs/2026-04-27-mariadb-healthcheck-hardening-design.md` — see PR 1 section.

---

## Task 0: Create feature branch

**Files:** none (git operation only).

- [ ] **Step 0.1: Confirm clean working tree**

Run:
```bash
git -C /data/git/private/mariadb-healthcheck status --short
```

Expected: only `?? .golangci.bck.yml` (the stale lint backup that PR 1 will delete in Task 5). No staged or modified tracked files.

- [ ] **Step 0.2: Create and check out branch**

Run:
```bash
git -C /data/git/private/mariadb-healthcheck checkout -b pr/1-refactor-single-module
git -C /data/git/private/mariadb-healthcheck branch --show-current
```

Expected output: `pr/1-refactor-single-module`.

---

## Task 1: Collapse to a single Go module

**Files:**
- Create: `go.mod`, `go.sum` at repo root
- Delete: `cmd/healthcheck/go.mod`, `cmd/healthcheck/go.sum`, `internal/mariadb/go.mod`, `internal/mariadb/go.sum`, `go.work`, `go.work.sum`
- Modify imports in:
  - `cmd/healthcheck/factor.go`
  - `cmd/healthcheck/handler.go`
  - `cmd/healthcheck/types.go`
  - `internal/mariadb/connection_test.go`
  - `internal/mariadb/queries_test.go`

- [ ] **Step 1.1: Delete the old module + workspace files**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
git rm cmd/healthcheck/go.mod cmd/healthcheck/go.sum \
       internal/mariadb/go.mod internal/mariadb/go.sum \
       go.work go.work.sum
```

Expected: 6 files staged for deletion.

- [ ] **Step 1.2: Create root `go.mod`**

Create `/data/git/private/mariadb-healthcheck/go.mod` with this content:

```go
module github.com/richie-tt/mariadb-healthcheck

go 1.25

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/go-sql-driver/mysql v1.9.3
	github.com/google/uuid v1.6.0
	github.com/stretchr/testify v1.11.1
)
```

Indirect deps will be added by `go mod tidy` in Step 1.4.

- [ ] **Step 1.3: Update import paths in 5 files**

In `/data/git/private/mariadb-healthcheck/cmd/healthcheck/factor.go`, replace:
```go
"mariadb"
```
with:
```go
"github.com/richie-tt/mariadb-healthcheck/internal/mariadb"
```

Apply the same replacement in:
- `/data/git/private/mariadb-healthcheck/cmd/healthcheck/handler.go`
- `/data/git/private/mariadb-healthcheck/cmd/healthcheck/types.go`
- `/data/git/private/mariadb-healthcheck/internal/mariadb/connection_test.go`
- `/data/git/private/mariadb-healthcheck/internal/mariadb/queries_test.go`

- [ ] **Step 1.4: Run `go mod tidy` to materialize `go.sum`**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
go mod tidy
```

Expected: creates `go.sum` at repo root, no errors. The `require` block in `go.mod` may be reformatted into `require (...)` with indirect deps appended.

- [ ] **Step 1.5: Run all tests from the new root**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
go test ./...
```

Expected output:
```
ok  	github.com/richie-tt/mariadb-healthcheck/cmd/healthcheck	0.0XXs
ok  	github.com/richie-tt/mariadb-healthcheck/internal/mariadb	0.0XXs
```

Both packages PASS. If a package shows `[no test files]` instead, that's also fine for any package without tests, but both packages above have tests.

- [ ] **Step 1.6: Run `golangci-lint`**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
golangci-lint run ./...
```

Expected: exit code 0, no findings. If lint surfaces existing issues unrelated to this refactor, note them in the PR description but do not fix them in PR 1.

- [ ] **Step 1.7: Stage and commit**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
git add -A
git status --short
```

Expected status (deletions, additions, modifications):
```
D  cmd/healthcheck/go.mod
D  cmd/healthcheck/go.sum
D  go.work
D  go.work.sum
D  internal/mariadb/go.mod
D  internal/mariadb/go.sum
A  go.mod
A  go.sum
M  cmd/healthcheck/factor.go
M  cmd/healthcheck/handler.go
M  cmd/healthcheck/types.go
M  internal/mariadb/connection_test.go
M  internal/mariadb/queries_test.go
?? .golangci.bck.yml
```

Then commit:
```bash
git commit -m "$(cat <<'EOF'
refactor: collapse to single Go module

Replace dual go.mod / go.work setup with a single module rooted at
github.com/richie-tt/mariadb-healthcheck. Update internal import paths.
No behavior change.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Replace `Query` methods with package-level functions; drop `Query` type

**Files:**
- Modify: `internal/mariadb/queries.go`
- Modify: `internal/mariadb/queries_test.go`
- Modify: `internal/mariadb/types.go`
- Modify: `cmd/healthcheck/handler.go`

**Goal of this task:** functions take `value string` directly; `Query` struct deleted; handler calls the functions directly. Behavior unchanged — same SQL, same errors.

- [ ] **Step 2.1: Rewrite `internal/mariadb/queries.go`**

Overwrite `/data/git/private/mariadb-healthcheck/internal/mariadb/queries.go` with:

```go
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
```

- [ ] **Step 2.2: Update `internal/mariadb/types.go`**

Overwrite `/data/git/private/mariadb-healthcheck/internal/mariadb/types.go` with:

```go
package mariadb

// Connection is a struct that contains required information to connect to a database
type Connection struct {
	Driver   string `env:"DB_DRIVER"`
	Database string `env:"DB_NAME"`
	Host     string `env:"DB_HOST"`
	Password string `env:"DB_PASSWORD"`
	Port     string `env:"DB_PORT"`
	User     string `env:"DB_USER"`
}
```

(`Query` struct removed.)

- [ ] **Step 2.3: Rewrite `internal/mariadb/queries_test.go`**

Overwrite `/data/git/private/mariadb-healthcheck/internal/mariadb/queries_test.go` with:

```go
package mariadb_test

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/richie-tt/mariadb-healthcheck/internal/mariadb"
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

		err = mariadb.InsertRow(t.Context(), db, "1")

		require.NoError(t, mock.ExpectationsWereMet())
		require.Error(t, err)
		assert.ErrorContains(t, err, "InsertRow")
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

		err = mariadb.InsertRow(t.Context(), db, "1")

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

		row, err := mariadb.SelectRow(t.Context(), db, "1")

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

		_, err = mariadb.SelectRow(t.Context(), db, "1")

		require.NoError(t, mock.ExpectationsWereMet())
		require.Error(t, err)
		assert.ErrorContains(t, err, "SelectRow")
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

		err = mariadb.DeleteRow(t.Context(), db, "1")

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

		err = mariadb.DeleteRow(t.Context(), db, "1")

		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
		assert.ErrorContains(t, err, "DeleteRow")
	})
}
```

- [ ] **Step 2.4: Update `cmd/healthcheck/handler.go` to call functions directly**

Replace the SQL-call sites in `/data/git/private/mariadb-healthcheck/cmd/healthcheck/handler.go`:

Find:
```go
	query := mariadb.Query{
		Value: c.ID.String(),
	}

	if err := query.InsertRow(ctx, c.DBInterface); err != nil {
```

Replace with:
```go
	if err := mariadb.InsertRow(ctx, c.DBInterface, c.ID.String()); err != nil {
```

Find:
```go
	row, err := query.SelectRow(ctx, c.DBInterface)
```

Replace with:
```go
	row, err := mariadb.SelectRow(ctx, c.DBInterface, c.ID.String())
```

Find:
```go
		if err := query.DeleteRow(ctx, c.DBInterface); err != nil {
```

Replace with:
```go
		if err := mariadb.DeleteRow(ctx, c.DBInterface, c.ID.String()); err != nil {
```

Verify the file no longer references `query :=` or `mariadb.Query`:
```bash
grep -n "mariadb.Query\|query :=" /data/git/private/mariadb-healthcheck/cmd/healthcheck/handler.go
```
Expected: no output.

- [ ] **Step 2.5: Run tests**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
go test ./...
```

Expected: both packages PASS.

- [ ] **Step 2.6: Run lint**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
golangci-lint run ./...
```

Expected: exit 0, no findings.

- [ ] **Step 2.7: Commit**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
git add internal/mariadb/queries.go internal/mariadb/queries_test.go \
        internal/mariadb/types.go cmd/healthcheck/handler.go
git commit -m "$(cat <<'EOF'
refactor: replace mariadb.Query with package-level functions

Drop the Query struct in favor of plain functions
InsertRow/SelectRow/DeleteRow that take the value as a parameter. Same
SQL, same errors, no behavior change. Handler updated to call functions
directly.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Add sentinel errors and `RunCheck` (TDD)

**Files:**
- Create: `internal/mariadb/check.go`
- Create: `internal/mariadb/check_test.go`

**Goal of this task:** introduce `RunCheck` orchestration function with sentinel errors. Handler is *not* yet wired to it (Task 4 does that). After this task `go test ./...` still passes; `RunCheck` is exercised by its new tests but unused in production code.

- [ ] **Step 3.1: Write the failing test file**

Create `/data/git/private/mariadb-healthcheck/internal/mariadb/check_test.go`:

```go
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
	const id = "test-id"

	t.Run("should succeed when delete is enabled", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(id).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(id).
			WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow(id))
		mock.ExpectExec("DELETE FROM status WHERE uuid = ?").
			WithArgs(id).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err = mariadb.RunCheck(t.Context(), db, id, true)

		assert.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("should succeed when delete is disabled", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(id).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(id).
			WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow(id))

		err = mariadb.RunCheck(t.Context(), db, id, false)

		assert.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("should return ErrInsert on insert failure", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(id).
			WillReturnError(errors.New("insert failed"))

		err = mariadb.RunCheck(t.Context(), db, id, true)

		require.Error(t, err)
		assert.True(t, errors.Is(err, mariadb.ErrInsert))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("should return ErrSelect on select failure", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(id).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(id).
			WillReturnError(errors.New("select failed"))

		err = mariadb.RunCheck(t.Context(), db, id, true)

		require.Error(t, err)
		assert.True(t, errors.Is(err, mariadb.ErrSelect))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("should return ErrValidate when value mismatches", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(id).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(id).
			WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow("different"))

		err = mariadb.RunCheck(t.Context(), db, id, true)

		require.Error(t, err)
		assert.True(t, errors.Is(err, mariadb.ErrValidate))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("should return ErrDelete on delete failure", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
			WithArgs(id).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
			WithArgs(id).
			WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow(id))
		mock.ExpectExec("DELETE FROM status WHERE uuid = ?").
			WithArgs(id).
			WillReturnError(errors.New("delete failed"))

		err = mariadb.RunCheck(t.Context(), db, id, true)

		require.Error(t, err)
		assert.True(t, errors.Is(err, mariadb.ErrDelete))
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
```

- [ ] **Step 3.2: Run tests to confirm compile failure**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
go test ./internal/mariadb 2>&1 | head -20
```

Expected: compile errors mentioning `mariadb.RunCheck`, `mariadb.ErrInsert`, `mariadb.ErrSelect`, `mariadb.ErrValidate`, `mariadb.ErrDelete` undefined.

- [ ] **Step 3.3: Implement `RunCheck` and sentinels**

Create `/data/git/private/mariadb-healthcheck/internal/mariadb/check.go`:

```go
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
// sequence using id as the UUID-shaped value written to the status table.
// On failure it returns one of the sentinel errors above wrapped with the
// underlying cause.
func RunCheck(ctx context.Context, db *sql.DB, id string, deleteRow bool) error {
	if err := InsertRow(ctx, db, id); err != nil {
		slog.ErrorContext(ctx, "failed to insert row", "error", err)
		return fmt.Errorf("%w: %v", ErrInsert, err)
	}

	slog.Debug( //nolint:G706 // UUID has fixed format
		"Executed query to insert row",
		"UUID", id,
	)

	row, err := SelectRow(ctx, db, id)
	if err != nil {
		slog.ErrorContext(ctx, "failed to select row", "error", err)
		return fmt.Errorf("%w: %v", ErrSelect, err)
	}

	slog.Debug( //nolint:G706 // UUID has fixed format
		"Executed query to select row",
		"UUID", id,
	)

	var value string
	if err := row.Scan(&value); err != nil {
		return fmt.Errorf("%w: %v", ErrScan, err)
	}

	if value != id {
		slog.Error( //nolint:G706 // UUID has fixed format
			"Value is not the same",
			"expected", id,
			"got", value,
		)

		return ErrValidate
	}

	if deleteRow {
		if err := DeleteRow(ctx, db, id); err != nil {
			slog.ErrorContext(ctx, "failed to delete row", "error", err)
			return fmt.Errorf("%w: %v", ErrDelete, err)
		}

		slog.Debug( //nolint:G706 // UUID has fixed format
			"Executed query to delete row",
			"UUID", id,
		)
	}

	return nil
}
```

- [ ] **Step 3.4: Run tests to verify they pass**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
go test ./internal/mariadb -v -run TestRunCheck
```

Expected: all six sub-tests PASS. Then run the full suite:
```bash
go test ./...
```
Expected: both packages PASS.

- [ ] **Step 3.5: Run lint**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
golangci-lint run ./...
```

Expected: exit 0, no findings.

- [ ] **Step 3.6: Commit**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
git add internal/mariadb/check.go internal/mariadb/check_test.go
git commit -m "$(cat <<'EOF'
feat(mariadb): add RunCheck and sentinel errors

Introduce mariadb.RunCheck to orchestrate INSERT -> SELECT -> DELETE
in one place, returning sentinel errors (ErrInsert, ErrSelect, ErrScan,
ErrValidate, ErrDelete) so callers can map failures to user messages
without parsing strings. Not yet wired to the handler — the next commit
swaps the handler to use it.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Wire the handler to `RunCheck`

**Files:**
- Modify: `cmd/healthcheck/handler.go`
- Modify: `cmd/healthcheck/handler_internal_test.go` (no logic change, but assertions remain valid)

**Goal of this task:** the handler delegates the SQL sequence to `mariadb.RunCheck`. Status codes and body strings remain byte-identical. Existing handler tests must pass without modification (they exercise the SQL path through sqlmock, which still works because `RunCheck` uses the same SQL).

- [ ] **Step 4.1: Rewrite `cmd/healthcheck/handler.go`**

Overwrite `/data/git/private/mariadb-healthcheck/cmd/healthcheck/handler.go` with:

```go
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
```

- [ ] **Step 4.2: Run handler tests**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
go test ./cmd/healthcheck -v -run TestHealthHandler
```

Expected: all seven sub-tests PASS (`should generate uuid if not set`, `should return failed to insert row`, `should return failed to select row`, `should return failed to validate row`, `should return OK, when clean table is false`, `should return failed to delete row`, `should return OK, when clean table is true`).

If the `should generate uuid if not set` test fails: it currently asserts `failed to insert row` because no mock expectations are set, so any `db.ExecContext` against the empty mock returns an error. The new code path produces the same outcome via `mariadb.RunCheck` → `mariadb.InsertRow` → mock returns error → handler maps `ErrInsert` to body "failed to insert row". The assertion should still hold.

- [ ] **Step 4.3: Run full test suite**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
go test ./...
```

Expected: both packages PASS.

- [ ] **Step 4.4: Run lint**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
golangci-lint run ./...
```

Expected: exit 0, no findings.

- [ ] **Step 4.5: Verify byte-identical HTTP behavior with a manual diff sanity check**

Confirm the final body strings match the originals by grepping the new handler:
```bash
grep -E '"failed to (insert|select|scan|validate|delete) row"|"OK"' /data/git/private/mariadb-healthcheck/cmd/healthcheck/handler.go
```

Expected output (5 error strings + 1 "OK"):
```
		writeBody(w, "OK")
		writeBody(w, "failed to insert row")
		writeBody(w, "failed to select row")
		writeBody(w, "failed to scan row")
		writeBody(w, "failed to validate row")
		writeBody(w, "failed to delete row")
```

- [ ] **Step 4.6: Commit**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
git add cmd/healthcheck/handler.go
git commit -m "$(cat <<'EOF'
refactor(handler): delegate SQL sequence to mariadb.RunCheck

Replace the inlined INSERT -> SELECT -> DELETE flow with a single call
to mariadb.RunCheck. Map sentinel errors back to the existing body
strings so HTTP status codes and bodies remain byte-identical.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Delete stale lint backup

**Files:**
- Delete: `.golangci.bck.yml`

- [ ] **Step 5.1: Remove the file**

The file is currently untracked. Delete it from the working tree:
```bash
rm /data/git/private/mariadb-healthcheck/.golangci.bck.yml
```

- [ ] **Step 5.2: Verify clean tree**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
git status --short
```

Expected: empty output (no `??` line for `.golangci.bck.yml` anymore, no other changes).

(No commit needed — the file was never tracked.)

---

## Task 6: Final verification

- [ ] **Step 6.1: Full test + lint + build**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
go test ./... && \
  golangci-lint run ./... && \
  go build -o /tmp/healthcheck-pr1 ./cmd/healthcheck && \
  ls -la /tmp/healthcheck-pr1
```

Expected:
- All tests PASS.
- golangci-lint exits 0.
- Binary `/tmp/healthcheck-pr1` is produced.

- [ ] **Step 6.2: Verify commit log**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
git log --oneline master..HEAD
```

Expected: four commits in this order, top-to-bottom (newest first):
```
<sha> refactor(handler): delegate SQL sequence to mariadb.RunCheck
<sha> feat(mariadb): add RunCheck and sentinel errors
<sha> refactor: replace mariadb.Query with package-level functions
<sha> refactor: collapse to single Go module
```

- [ ] **Step 6.3: Cleanup the binary**

Run:
```bash
rm /tmp/healthcheck-pr1
```

- [ ] **Step 6.4: Stop and report**

The branch `pr/1-refactor-single-module` is now ready for PR creation. Report back with:
- The branch name.
- The four commit SHAs.
- Output of `go test ./...`.
- Output of `golangci-lint run ./...`.

PR creation, push, and merge are user-driven actions — do **not** push or open a PR without explicit instruction.

---

## Notes on behavior preservation

This PR aims at byte-identical HTTP behavior. A few subtle points:

1. **`slog` message location.** The `slog.ErrorContext` calls for failed INSERT/SELECT/DELETE were previously emitted from `cmd/healthcheck/handler.go`. They now live in `internal/mariadb/check.go`. Same message text, same `error` field, same level. Observable output via `LOG_LEVEL=info` is identical.
2. **Missing slog on Scan.** The current handler does **not** call `slog.ErrorContext` when `row.Scan` fails. The new `RunCheck` preserves that gap deliberately. PR 2 fixes it.
3. **`ErrValidate` is unreachable in production.** With `WHERE uuid = ?`, a returned row can only have the matching value. The mock-driven test exists as a guard until PR 2 removes both the comparison and the test.
4. **No retry, no goroutine, no caching.** Per the spec, the check stays synchronous and on-demand.
