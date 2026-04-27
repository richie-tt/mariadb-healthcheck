# PR 2: Security + Logic Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tighten DSN handling and connection pool, surface insecure defaults, fix dead/inconsistent code paths in the health-check flow, harden the HTTP server, and add graceful shutdown — without altering the wire contract of `/health` (status codes and body strings stay identical).

**Architecture:** `internal/mariadb.ConnectDB` builds the DSN via `mysql.Config{}` instead of `fmt.Sprintf` and tunes the connection pool. `cmd/healthcheck/main.go` adds `ReadHeaderTimeout`, `IdleTimeout`, and signal-driven graceful shutdown around `http.Server`. `cmd/healthcheck/factor.go` requires `DB_PASSWORD`, drops the silent default, and replaces the per-field "if empty default; else parse" pattern with three small string-based helpers (`or`, `intOr`, `boolOr`) consumed by `parseEnv`. `internal/mariadb/check.go` distinguishes `sql.ErrNoRows` (becomes `ErrValidate`) from other scan failures (`ErrScan`), removes the unreachable `value != id` block, and stops emitting the per-stage slog lines so the handler becomes the single error-logging boundary. `cmd/healthcheck/handler.go` always generates a fresh UUID per request, sets `Content-Type: text/plain; charset=utf-8`, and emits one `slog.ErrorContext` on failure.

**Tech Stack:** Go 1.25, `database/sql`, `github.com/go-sql-driver/mysql` v1.9.3, `github.com/google/uuid` v1.6.0, `github.com/DATA-DOG/go-sqlmock` v1.5.2, `github.com/stretchr/testify` v1.11.1.

**Spec:** `docs/superpowers/specs/2026-04-27-mariadb-healthcheck-hardening-design.md` — see PR 2 section.

**Branch:** `pr/2-security-logic` (already created from PR 1 HEAD `45362a1`). When PR 1 merges to master, rebase this branch onto the new master before opening the PR.

---

## File structure (changes by file)

| File | Concerns | Changes |
| --- | --- | --- |
| `internal/mariadb/connection.go` | DSN, validation, pool | S1 (DSN via `mysql.Config`), S6 (drop `url.Parse` host check), S7 (pool tuning) |
| `internal/mariadb/connection_test.go` | tests for the above | DSN round-trip with special chars; remove obsolete invalid-host test; pool config assertion |
| `cmd/healthcheck/main.go` | server lifecycle | S5 (`ReadHeaderTimeout`, `IdleTimeout`), L6 (graceful shutdown) |
| `cmd/healthcheck/main_test.go` | server tests | assert new server fields; new graceful-shutdown test |
| `cmd/healthcheck/const.go` | defaults | S2 (drop `defaultDBPassword`) |
| `cmd/healthcheck/factor.go` | env parsing | S2 (require `DB_PASSWORD`), L4 (helpers `envOr`, `envInt`, `envBool`; collapse `getEnv`+`parseEnv`) |
| `cmd/healthcheck/factor_internal_test.go` | env parse tests | new "missing password" test; tests adapted for collapsed loader |
| `cmd/healthcheck/types.go` | config struct | L1 (drop `ID uuid.UUID` from `config`) |
| `internal/mariadb/check.go` | check flow | L2 (ErrNoRows → `ErrValidate`; remove dead mismatch block), L3 (remove per-stage slog) |
| `internal/mariadb/check_test.go` | RunCheck tests | new "row missing" test; replace ErrScan setup; remove value-mismatch test |
| `cmd/healthcheck/handler.go` | HTTP handler | L1 (drop `uuid.Nil` block, generate per request), L3 (single boundary log), L5 (Content-Type) |
| `cmd/healthcheck/handler_internal_test.go` | handler tests | tests no longer set `config.ID`; new Content-Type assertion |
| `README.md` | docs | mark `DB_PASSWORD` Required = Yes |

---

## Task 0: Confirm branch state

**Files:** none.

- [ ] **Step 0.1: Confirm we're on the right branch with a clean tree**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
git branch --show-current
git log --oneline master..HEAD | head -1
git status --short
```

Expected:
- branch is `pr/2-security-logic`
- the most recent commit is `45362a1 chore: preserve filippo.io/edwards25519 v1.1.1 indirect pin` (the last commit from PR 1)
- working tree is clean (empty `git status --short`)

If the branch doesn't exist, create it from `pr/1-refactor-single-module` HEAD:
```bash
git checkout pr/1-refactor-single-module
git checkout -b pr/2-security-logic
```

- [ ] **Step 0.2: Confirm tests + lint baseline**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
go test ./...
golangci-lint run ./...
```

Expected: both packages PASS, lint reports `0 issues.`

---

## Task 1: Connection — DSN via `mysql.Config`, drop `url.Parse`, pool tuning (S1 + S6 + S7)

**Files:**
- Modify: `internal/mariadb/connection.go`
- Modify: `internal/mariadb/connection_test.go`

**Goal:** Build the DSN with the driver's `mysql.Config{}` so passwords with `@:/?#` no longer break parsing. Drop the misleading `url.Parse(c.Host)` host check. Tune the `*sql.DB` pool for sidecar usage. The `Connection.Validate` method continues to validate the obvious empty/range cases.

- [ ] **Step 1.1: Write failing tests for DSN special chars + pool config**

Append to `/data/git/private/mariadb-healthcheck/internal/mariadb/connection_test.go` (do NOT delete the existing tests yet — that's Step 1.3):

```go
func TestConnectDB_DSN_handlesSpecialChars(t *testing.T) {
	conn := mariadb.Connection{
		Driver:   "mysql",
		Database: "healthcheck",
		Host:     "127.0.0.1",
		Password: "p@ss:w/o?rd#",
		Port:     "3306",
		User:     "user",
	}

	db, err := conn.ConnectDB()
	require.NoError(t, err)
	defer db.Close()

	// The pool was configured for sidecar load; verify the cap.
	stats := db.Stats()
	assert.Equal(t, 2, stats.MaxOpenConnections, "MaxOpenConns should be 2")
}
```

- [ ] **Step 1.2: Run the new test to confirm it fails**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
go test ./internal/mariadb -run TestConnectDB_DSN_handlesSpecialChars -v
```

Expected: FAIL on the `MaxOpenConnections` assertion (pool not configured yet) — or compile-error on `mariadb.Connection` if the Driver field isn't set in your version.

- [ ] **Step 1.3: Rewrite `connection.go`**

Overwrite `/data/git/private/mariadb-healthcheck/internal/mariadb/connection.go` with:

```go
// Package mariadb provides a connection to a MariaDB database.
package mariadb

import (
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
)

const (
	dbConnectTimeout  = 5 * time.Second
	dbMaxOpenConns    = 2
	dbMaxIdleConns    = 1
	dbConnMaxLifetime = 5 * time.Minute
	dbConnMaxIdleTime = 1 * time.Minute
)

// Validate validates the connection
func (c *Connection) Validate() error {
	if c.User == "" {
		return fmt.Errorf("user is empty")
	}

	if c.Password == "" {
		return fmt.Errorf("password is empty")
	}

	if c.Host == "" {
		return fmt.Errorf("host is empty")
	}

	if c.Port == "" {
		return fmt.Errorf("port is empty")
	}

	if c.Database == "" {
		return fmt.Errorf("database is empty")
	}

	port, err := strconv.Atoi(c.Port)
	if err != nil {
		return fmt.Errorf("invalid port: %w", err)
	}

	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}

	return nil
}

// ConnectDB connects to the database using a DSN built via mysql.Config so
// that special characters in the password are escaped correctly.
func (c Connection) ConnectDB() (*sql.DB, error) {
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate connection: %w", err)
	}

	cfg := mysql.NewConfig()
	cfg.User = c.User
	cfg.Passwd = c.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(c.Host, c.Port)
	cfg.DBName = c.Database
	cfg.ParseTime = true
	cfg.Timeout = dbConnectTimeout

	db, err := sql.Open(c.Driver, cfg.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db.SetMaxOpenConns(dbMaxOpenConns)
	db.SetMaxIdleConns(dbMaxIdleConns)
	db.SetConnMaxLifetime(dbConnMaxLifetime)
	db.SetConnMaxIdleTime(dbConnMaxIdleTime)

	return db, nil
}
```

Notes:
- `url.Parse(c.Host)` is removed; the original "should return error if host is invalid" test in `connection_test.go` was relying on that — Step 1.4 removes that test.
- The existing `Validate` rules for empty fields and port range stay. `Connection.Driver` defaults to `"mysql"` from the caller's perspective; `sql.Open` will reject an unregistered driver.

- [ ] **Step 1.4: Update existing tests in `connection_test.go`**

Open `/data/git/private/mariadb-healthcheck/internal/mariadb/connection_test.go` and:

1. **Remove** the sub-test named `should return error if host is invalid` from `TestValidate` — the `url.Parse` check no longer exists, and the inputs `http://:[invalid]` are now passed verbatim to `mysql.Config.Addr`, which doesn't reject them at validation time.

2. **Keep** all other `TestValidate` sub-tests (empty user / empty password / empty host / empty port / empty database / port not a number / port out of range) — they continue to assert on `Validate()` directly.

3. **Update** `TestConnectDB`'s "should return error if connection is invalid" sub-test — it currently expects the error to contain "failed to validate connection". That still works.

4. **Update** `TestConnectDB`'s "should return error if connection fails" sub-test — change the `Driver: "unknown"` case to expect `"failed to connect to database"` (unchanged) and also add `Password: "password"` if missing (the new code path goes through Validate first, which now requires Password).

5. **Update** `TestConnectDB`'s "should run successfully" sub-test — also ensures all required fields are set (User/Password/Host/Port/Database/Driver=`"mysql"`).

The full updated `TestConnectDB` block should look like:

```go
func TestConnectDB(t *testing.T) {
	t.Run("should return error if connection is invalid", func(t *testing.T) {
		conn := &mariadb.Connection{}

		_, err := conn.ConnectDB()

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to validate connection")
	})

	t.Run("should return error if connection fails", func(t *testing.T) {
		conn := &mariadb.Connection{
			User:     "user",
			Password: "password",
			Host:     "host",
			Port:     "3306",
			Database: "database",
			Driver:   "unknown",
		}

		_, err := conn.ConnectDB()

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to connect to database")
	})

	t.Run("should run successfully", func(t *testing.T) {
		conn := &mariadb.Connection{
			User:     "user",
			Password: "password",
			Host:     "host",
			Port:     "3306",
			Database: "database",
			Driver:   "mysql",
		}

		_, err := conn.ConnectDB()

		require.NoError(t, err)
	})
}
```

- [ ] **Step 1.5: Run the full `internal/mariadb` test suite**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
go test ./internal/mariadb -v
```

Expected: all tests PASS, including the new `TestConnectDB_DSN_handlesSpecialChars`.

- [ ] **Step 1.6: Lint**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
golangci-lint run ./...
```

Expected: 0 issues.

- [ ] **Step 1.7: Commit**

```bash
cd /data/git/private/mariadb-healthcheck
git add internal/mariadb/connection.go internal/mariadb/connection_test.go
git commit -m "$(cat <<'EOF'
refactor(mariadb): build DSN via mysql.Config; tune pool; drop url.Parse

Replace fmt.Sprintf-based DSN construction with mysql.Config.FormatDSN
so that passwords containing @, :, /, ?, # parse correctly. Tune the
connection pool for sidecar load (max 2 open, 1 idle, 5m lifetime,
1m idle timeout) and drop the misleading url.Parse host validation.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: HTTP server timeouts (S5)

**Files:**
- Modify: `cmd/healthcheck/main.go`
- Modify: `cmd/healthcheck/main_test.go`
- Modify: `cmd/healthcheck/const.go`

- [ ] **Step 2.1: Add the new timeout constants to `const.go`**

Append to `/data/git/private/mariadb-healthcheck/cmd/healthcheck/const.go`'s `const` block:

```go
	httpReadHeaderTimeout = time.Second * 5
	httpIdleTimeout       = time.Second * 30
```

- [ ] **Step 2.2: Update `setupServer` in `main.go`**

In `/data/git/private/mariadb-healthcheck/cmd/healthcheck/main.go`, replace:

```go
	return &http.Server{
		Addr:         fmt.Sprintf(":%d", config.HealthPort),
		Handler:      mux,
		ReadTimeout:  httpReadTimeout,
		WriteTimeout: httpWriteTimeout,
	}
```

with:

```go
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", config.HealthPort),
		Handler:           mux,
		ReadTimeout:       httpReadTimeout,
		ReadHeaderTimeout: httpReadHeaderTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
	}
```

- [ ] **Step 2.3: Update `main_test.go`**

In `/data/git/private/mariadb-healthcheck/cmd/healthcheck/main_test.go`, the `TestSetupServer` "should setup server" sub-test currently asserts:

```go
		assert.Equal(t, ":8080", server.Addr)
		assert.NotNil(t, server.Handler)
		assert.Equal(t, httpReadTimeout, server.ReadTimeout)
		assert.Equal(t, httpWriteTimeout, server.WriteTimeout)
```

Add two more assertions immediately after `WriteTimeout`:

```go
		assert.Equal(t, httpReadHeaderTimeout, server.ReadHeaderTimeout)
		assert.Equal(t, httpIdleTimeout, server.IdleTimeout)
```

- [ ] **Step 2.4: Run tests**

```bash
cd /data/git/private/mariadb-healthcheck
go test ./cmd/healthcheck -v -run TestSetupServer
```

Expected: PASS for both sub-tests.

- [ ] **Step 2.5: Run full suite + lint**

```bash
cd /data/git/private/mariadb-healthcheck
go test ./...
golangci-lint run ./...
```

Expected: PASS, 0 issues.

- [ ] **Step 2.6: Commit**

```bash
cd /data/git/private/mariadb-healthcheck
git add cmd/healthcheck/main.go cmd/healthcheck/main_test.go cmd/healthcheck/const.go
git commit -m "$(cat <<'EOF'
feat(server): add ReadHeaderTimeout and IdleTimeout

Set ReadHeaderTimeout=5s and IdleTimeout=30s on http.Server to harden
against slow header attacks and bound idle keep-alive connections.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Graceful shutdown (L6)

**Files:**
- Modify: `cmd/healthcheck/main.go`
- Modify: `cmd/healthcheck/main_test.go`
- Modify: `cmd/healthcheck/const.go`

**Goal:** Wire `signal.NotifyContext` for `os.Interrupt` and `syscall.SIGTERM`. On signal, call `server.Shutdown(shutdownCtx)` with a 5-second timeout, then close the DB. Replace the dead `defer server.Close()` with the new flow.

- [ ] **Step 3.1: Add the shutdown timeout constant**

Append to `/data/git/private/mariadb-healthcheck/cmd/healthcheck/const.go`'s `const` block:

```go
	shutdownTimeout = time.Second * 5
```

- [ ] **Step 3.2: Rewrite `run()` in `main.go`**

In `/data/git/private/mariadb-healthcheck/cmd/healthcheck/main.go`, replace the entire `run` function (and its imports) so the file becomes:

```go
// Package main is the entry point for the healthcheck command.
// It parses the environment variables and starts the HTTP server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/go-sql-driver/mysql"
)

var (
	// Version is the application version
	Version = ""
	// BuildDate is the date the application was built
	BuildDate = ""
	// Commit is the git commit hash the application was built from
	Commit = ""
)

func main() {
	if err := run(); err != nil {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}
}

func (c config) getLogLevel() (slog.Level, error) {
	switch c.LogLevel {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level: %s", c.LogLevel)
	}
}

func setupServer(config config) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", config.healthHandler)

	return &http.Server{
		Addr:              fmt.Sprintf(":%d", config.HealthPort),
		Handler:           mux,
		ReadTimeout:       httpReadTimeout,
		ReadHeaderTimeout: httpReadHeaderTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
	}
}

func run() error {
	slog.Info(
		"starting healthcheck",
		"version", Version,
		"commit", Commit,
		"build_date", BuildDate,
	)

	env := getEnv()

	config, err := env.parseEnv()
	if err != nil {
		return fmt.Errorf("failed to parse environment: %w", err)
	}

	db, err := config.Connection.ConnectDB()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	config.DBInterface = db

	server := setupServer(*config)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
		}
	}()

	slog.Info(
		"starting health check server",
		"port", config.HealthPort,
	)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}
```

Note: the `defer server.Close()` is removed (it was dead — `ListenAndServe` only returns when the server is already done). `http.ErrServerClosed` is the expected return after `Shutdown`, so it must not be treated as an error.

- [ ] **Step 3.3: Add a graceful-shutdown test**

Append to `/data/git/private/mariadb-healthcheck/cmd/healthcheck/main_test.go`:

```go
func TestRun_gracefulShutdownOnSIGTERM(t *testing.T) {
	t.Setenv("DB_PASSWORD", "test")
	t.Setenv("HEALTH_PORT", "0") // os-assigned, but run() builds a real :0 listener

	// We can't easily exercise the full run() path in a unit test (it dials the
	// real DB). Instead verify that an http.Server returned by setupServer
	// supports Shutdown without error.
	srv := setupServer(config{HealthPort: 0})

	// ListenAndServe in a goroutine; Shutdown should make it return cleanly.
	listenErr := make(chan error, 1)
	go func() {
		listenErr <- srv.ListenAndServe()
	}()

	// Give the listener a moment.
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.NoError(t, srv.Shutdown(ctx))
	require.ErrorIs(t, <-listenErr, http.ErrServerClosed)
}
```

Add the necessary imports if missing: `context`, `net/http`, `time`.

- [ ] **Step 3.4: Run tests**

```bash
cd /data/git/private/mariadb-healthcheck
go test ./cmd/healthcheck -v -run TestRun_gracefulShutdownOnSIGTERM
go test ./...
```

Expected: new test PASSes; full suite PASSes.

- [ ] **Step 3.5: Lint**

```bash
golangci-lint run ./...
```

Expected: 0 issues.

- [ ] **Step 3.6: Commit**

```bash
git add cmd/healthcheck/main.go cmd/healthcheck/main_test.go cmd/healthcheck/const.go
git commit -m "$(cat <<'EOF'
feat(server): add graceful shutdown on SIGINT/SIGTERM

Wire signal.NotifyContext to shut the http.Server down with a 5s
timeout when the process receives Interrupt or SIGTERM. Treat
http.ErrServerClosed as the expected post-shutdown return value, and
remove the dead defer server.Close().

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Require `DB_PASSWORD` (S2)

**Files:**
- Modify: `cmd/healthcheck/const.go`
- Modify: `cmd/healthcheck/factor.go`
- Modify: `cmd/healthcheck/factor_internal_test.go`
- Modify: `README.md`

**Goal:** Drop `defaultDBPassword`; `parseEnv` returns an error if `DB_PASSWORD` is empty. Keep `DB_USER` defaulting to `healthcheck`. Update README.

- [ ] **Step 4.1: Drop the default password from `const.go`**

In `/data/git/private/mariadb-healthcheck/cmd/healthcheck/const.go`, remove the line:

```go
	defaultDBPassword = "healthcheck"
```

- [ ] **Step 4.2: Update `factor.go`**

In `/data/git/private/mariadb-healthcheck/cmd/healthcheck/factor.go`, replace:

```go
	if config.Connection.Password == "" {
		config.Connection.Password = defaultDBPassword
	}
```

with:

```go
	if config.Connection.Password == "" {
		return nil, fmt.Errorf("DB_PASSWORD environment variable is required")
	}
```

- [ ] **Step 4.3: Update `factor_internal_test.go`**

In `/data/git/private/mariadb-healthcheck/cmd/healthcheck/factor_internal_test.go`:

1. **Add** a new sub-test under `TestParseEnv` — place it directly after the `should return default values for password` sub-test (which we will rename, see step 2):

```go
	t.Run("should return error when DB_PASSWORD is empty", func(t *testing.T) {
		t.Setenv(dbPassword, "")

		_, err := getEnv().parseEnv()

		require.Error(t, err)
		assert.ErrorContains(t, err, "DB_PASSWORD environment variable is required")
	})
```

2. **Remove** the existing sub-test named `should return default values for password` — there is no longer a default. Replace it with a happy-path sub-test that confirms an explicit password flows through:

```go
	t.Run("should accept an explicit DB_PASSWORD", func(t *testing.T) {
		t.Setenv(dbPassword, "explicit")

		parsedEnv, err := getEnv().parseEnv()

		require.NoError(t, err)
		assert.Equal(t, "explicit", parsedEnv.Connection.Password)
	})
```

3. **Update every other sub-test in `TestParseEnv` that calls `getEnv().parseEnv()`** — they all need a `t.Setenv(dbPassword, "test")` at the top of the sub-test, otherwise the new error path will trip them. Affected sub-tests:
   - `should return default values for database`
   - `should return default values for host`
   - `should return default values for port`
   - `should return default values for user`
   - `should return default values for logLevel`
   - `should return error for invalid logLevel`
   - `should return default values for healthPort`
   - `should return error for invalid healthPort`
   - `should return parsed custom values for healthPort`
   - `should return default values for cleanTable`
   - `should return error for invalid deleteRow`
   - `should return parsed custom values for deleteRow`

   (`should return all env variables` from `TestGetEnv` already sets all the env vars including `dbPassword`, so it's unaffected.)

   For each, add `t.Setenv(dbPassword, "test")` immediately after any other `t.Setenv` calls (or at the top if there are none).

- [ ] **Step 4.4: Update `README.md`**

In the env-variables table around line 33-42, change the `DB_PASSWORD` row:

From:
```
| DB_PASSWORD | No       | `healthcheck` | MariaDB user password.                                                                                                                              |
```

To:
```
| DB_PASSWORD | **Yes**  | _(none)_      | MariaDB user password. The container will refuse to start if this is unset.                                                                         |
```

- [ ] **Step 4.5: Run tests**

```bash
cd /data/git/private/mariadb-healthcheck
go test ./cmd/healthcheck -v -run TestParseEnv
```

Expected: all sub-tests PASS, including the new `should return error when DB_PASSWORD is empty`.

- [ ] **Step 4.6: Lint**

```bash
golangci-lint run ./...
```

Expected: 0 issues. If the linter complains that `defaultDBPassword` is unused but still declared, double-check Step 4.1 actually removed it.

- [ ] **Step 4.7: Commit**

```bash
git add cmd/healthcheck/const.go cmd/healthcheck/factor.go \
        cmd/healthcheck/factor_internal_test.go README.md
git commit -m "$(cat <<'EOF'
feat(security): require DB_PASSWORD; drop silent default

Refuse to start when DB_PASSWORD is unset rather than falling back to
the literal default "healthcheck". DB_USER keeps its default for
ergonomics — only the secret needs to be explicit. README updated.

This is a deliberate, security-motivated breaking change for any
deployment that was relying on the default password.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Simplify `parseEnv` with `or`/`intOr`/`boolOr` helpers (L4)

**Files:**
- Modify: `cmd/healthcheck/factor.go`
- Modify: `cmd/healthcheck/factor_internal_test.go`

**Goal:** Replace the per-field "if empty default; else parse" branching with three small string-based helpers. Keep the existing `getEnv` → `parseEnv` boundary so `TestGetEnv` and `TestParseEnv` continue to apply. The helpers operate on the string values that `getEnv` collected — no env keys are passed in.

- [ ] **Step 5.1: Rewrite `factor.go`**

Overwrite `/data/git/private/mariadb-healthcheck/cmd/healthcheck/factor.go` with:

```go
package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/richie-tt/mariadb-healthcheck/internal/mariadb"
)

// or returns value when it is non-empty, otherwise fallback.
func or(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}

// intOr parses value as an int; returns fallback when value is empty.
func intOr(value string, fallback int) (int, error) {
	if value == "" {
		return fallback, nil
	}

	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid integer %q: %w", value, err)
	}

	return n, nil
}

// boolOr parses value as a bool; returns fallback when value is empty.
func boolOr(value string, fallback bool) (bool, error) {
	if value == "" {
		return fallback, nil
	}

	b, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid bool %q: %w", value, err)
	}

	return b, nil
}

func getEnv() environment {
	return environment{
		Connection: mariadb.Connection{
			Database: os.Getenv(dbName),
			Driver:   "mysql",
			Host:     os.Getenv(dbHost),
			Password: os.Getenv(dbPassword),
			Port:     os.Getenv(dbPort),
			User:     os.Getenv(dbUser),
		},
		DeleteRow:  os.Getenv(deleteRow),
		HealthPort: os.Getenv(healthPort),
		LogLevel:   os.Getenv(logLevel),
	}
}

func (e environment) parseEnv() (*config, error) {
	if e.Connection.Password == "" {
		return nil, fmt.Errorf("DB_PASSWORD environment variable is required")
	}

	cfg := config{
		Connection: mariadb.Connection{
			Driver:   "mysql",
			Database: or(e.Connection.Database, defaultDBName),
			Host:     or(e.Connection.Host, defaultDBHost),
			Password: e.Connection.Password,
			Port:     or(e.Connection.Port, defaultDBPort),
			User:     or(e.Connection.User, defaultDBUser),
		},
		LogLevel: or(e.LogLevel, "info"),
	}

	level, err := cfg.getLogLevel()
	if err != nil {
		slog.Error(
			"failed to get log level, available levels: debug, info, warn, error",
			"error", err,
		)

		return nil, fmt.Errorf("failed to parse the log level: %w", err)
	}

	slog.SetDefault(
		slog.New(
			slog.NewTextHandler(
				os.Stdout,
				&slog.HandlerOptions{
					Level: level,
				},
			),
		),
	)

	port, err := intOr(e.HealthPort, 8080)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HealthPort: %w", err)
	}

	cfg.HealthPort = port

	clean, err := boolOr(e.DeleteRow, true)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DeleteRow: %w", err)
	}

	cfg.DeleteRow = clean

	if !clean {
		slog.Warn("delete row is disabled")
	}

	return &cfg, nil
}
```

Notes:
- `getEnv()` keeps its old signature (used by `TestGetEnv`).
- `parseEnv()` operates only on values already in `e environment` — no fresh `os.Getenv` calls. Helpers are pure: `or(value, fallback)`, `intOr(value, fallback)`, `boolOr(value, fallback)`. They take string values, not env keys.
- The previously emitted debug logs ("using default log level", "using default delete row", "parsed delete row", "using default health port") are dropped — they were noise. `slog.Warn("delete row is disabled")` is preserved because that's a real configuration signal.

- [ ] **Step 5.2: Update `factor_internal_test.go`**

Most of the `TestParseEnv` sub-tests still work because they set env vars and call `getEnv().parseEnv()`. Two adjustments:

1. The error messages for invalid `HEALTH_PORT` and `DELETE_ROW` were previously `"failed to parse HealthPort"` and `"failed to parse DeleteRow"`. They are unchanged.

2. The error message for an invalid log level was `"failed to parse the log level"`. Unchanged.

If any sub-test uses `assert.ErrorContains(t, err, "<message>")` against something other than the strings just listed, update accordingly. Run the tests; the runner will pinpoint mismatches.

- [ ] **Step 5.3: Run tests**

```bash
cd /data/git/private/mariadb-healthcheck
go test ./cmd/healthcheck -v -run TestParseEnv
```

Expected: all sub-tests PASS.

- [ ] **Step 5.4: Run full suite + lint**

```bash
go test ./...
golangci-lint run ./...
```

Expected: PASS, 0 issues.

- [ ] **Step 5.5: Commit**

```bash
git add cmd/healthcheck/factor.go cmd/healthcheck/factor_internal_test.go
git commit -m "$(cat <<'EOF'
refactor(factor): replace per-field branches with envOr/envInt/envBool

Collapse the "if empty default; else parse" pattern into three small
helpers and consolidate parseEnv so each field is a single readable
line. Behavior unchanged: same defaults, same error messages, same
validation order. Drops a handful of debug-level "using default …"
log lines that were noise rather than signal.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: ErrNoRows handling + remove dead value-mismatch (L2)

**Files:**
- Modify: `internal/mariadb/check.go`
- Modify: `internal/mariadb/check_test.go`

**Goal:** Treat `sql.ErrNoRows` from `Scan` as a meaningful signal — the row we just inserted is gone — and return `ErrValidate` (re-purposing the existing sentinel since the body string "failed to validate row" still applies). Other Scan failures keep returning `ErrScan`. Remove the unreachable `value != id` block.

- [ ] **Step 6.1: Rewrite the Scan + validate section in `check.go`**

In `/data/git/private/mariadb-healthcheck/internal/mariadb/check.go`, find this block (around lines 47-61):

```go
	var value string
	if err := row.Scan(&value); err != nil {
		return fmt.Errorf("%w: %v", ErrScan, err)
	}

	if value != uuid {
		slog.Error( //nolint:G706 // UUID has fixed format
			"Value is not the same",
			"expected", uuid,
			"got", value,
		)

		return ErrValidate
	}
```

Replace with:

```go
	var value string
	if err := row.Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: inserted row not found", ErrValidate)
		}

		return fmt.Errorf("%w: %v", ErrScan, err)
	}
```

(The `value` variable is no longer compared, but we still need to declare it for the `Scan` call. `var value string` followed by `_ = value` would silence "declared and not used" — but `Scan` reading into it is the use. Confirm `go vet ./...` doesn't complain.)

Note: the `slog` package import in `check.go` is no longer needed by this section, but is still used by other slog calls. Leave the import; Task 7 (consolidate logging) will clean it up.

- [ ] **Step 6.2: Update `check_test.go`**

In `/data/git/private/mariadb-healthcheck/internal/mariadb/check_test.go`:

1. **Replace** the `should return ErrValidate when value mismatches` sub-test with `should return ErrValidate when row is missing`:

   The current test sets up `mock.ExpectQuery(...).WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow("different"))` and expects `errors.Is(err, mariadb.ErrValidate)`. Since the value-mismatch path is gone, replace with an empty-rows setup that triggers `sql.ErrNoRows`:

   ```go
   	t.Run("should return ErrValidate when row is missing", func(t *testing.T) {
   		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
   		require.NoError(t, err)
   		defer db.Close()

   		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
   			WithArgs(uuid).
   			WillReturnResult(sqlmock.NewResult(1, 1))
   		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
   			WithArgs(uuid).
   			WillReturnRows(sqlmock.NewRows([]string{"uuid"}))

   		err = mariadb.RunCheck(t.Context(), db, uuid, true)

   		require.Error(t, err)
   		require.ErrorIs(t, err, mariadb.ErrValidate)
   		require.NoError(t, mock.ExpectationsWereMet())
   	})
   ```

2. **Replace** the `should return ErrScan on scan failure` sub-test setup. The current test uses empty rows (which now maps to `ErrValidate`). Use sqlmock's `RowError` to inject a non-`ErrNoRows` scan failure:

   ```go
   	t.Run("should return ErrScan on scan failure", func(t *testing.T) {
   		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
   		require.NoError(t, err)
   		defer db.Close()

   		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
   			WithArgs(uuid).
   			WillReturnResult(sqlmock.NewResult(1, 1))
   		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
   			WithArgs(uuid).
   			WillReturnRows(
   				sqlmock.NewRows([]string{"uuid"}).
   					AddRow(uuid).
   					RowError(0, errors.New("scan boom")),
   			)

   		err = mariadb.RunCheck(t.Context(), db, uuid, true)

   		require.Error(t, err)
   		require.ErrorIs(t, err, mariadb.ErrScan)
   		require.NoError(t, mock.ExpectationsWereMet())
   	})
   ```

- [ ] **Step 6.3: Run the RunCheck tests**

```bash
cd /data/git/private/mariadb-healthcheck
go test ./internal/mariadb -v -run TestRunCheck
```

Expected: all 7 sub-tests PASS, including the renamed `should return ErrValidate when row is missing` and the rewritten `should return ErrScan on scan failure`.

- [ ] **Step 6.4: Run full suite + lint**

```bash
go test ./...
golangci-lint run ./...
```

Expected: PASS, 0 issues.

- [ ] **Step 6.5: Commit**

```bash
git add internal/mariadb/check.go internal/mariadb/check_test.go
git commit -m "$(cat <<'EOF'
fix(check): treat sql.ErrNoRows as ErrValidate; drop unreachable check

The WHERE clause of the SELECT guarantees that a returned row matches
the inserted UUID, so the value != id comparison was unreachable in
production. Remove it.

In its place, distinguish sql.ErrNoRows from other Scan failures: the
former (the row vanished between INSERT and SELECT) is a real storage
anomaly and surfaces as ErrValidate; other Scan errors keep ErrScan.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Drop `config.ID`; always generate UUID per request; consolidate logging (L1 + L3)

**Files:**
- Modify: `cmd/healthcheck/types.go`
- Modify: `cmd/healthcheck/handler.go`
- Modify: `cmd/healthcheck/handler_internal_test.go`
- Modify: `internal/mariadb/check.go`

**Goal:** Remove the `ID uuid.UUID` field from `config` (it was dead code — value-receiver handler made the check unreachable). Generate a fresh UUID at the start of every `healthHandler` invocation. Move all per-stage `slog.ErrorContext` calls out of `RunCheck` into a single boundary log in the handler. Keep the slog.Debug "Executed query to …" calls in `RunCheck` (they document the SQL flow; not duplicated by the handler).

- [ ] **Step 7.1: Drop `ID` from `config` in `types.go`**

In `/data/git/private/mariadb-healthcheck/cmd/healthcheck/types.go`, remove:

```go
	ID          uuid.UUID
```

…from the `config` struct, and remove the now-unused import `"github.com/google/uuid"` (if it isn't used elsewhere in the file). The full file becomes:

```go
package main

import (
	"database/sql"

	"github.com/richie-tt/mariadb-healthcheck/internal/mariadb"
)

type environment struct {
	DeleteRow  string
	Connection mariadb.Connection
	HealthPort string
	LogLevel   string
}

type config struct {
	Connection  mariadb.Connection
	DBInterface *sql.DB
	DeleteRow   bool
	HealthPort  int
	LogLevel    string
}
```

- [ ] **Step 7.2: Rewrite `handler.go`**

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

func (c config) healthHandler(w http.ResponseWriter, r *http.Request) {
	id := uuid.New()

	slog.Debug( //nolint:G706 // UUID has fixed format
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
```

Notes:
- UUID generation is unconditional now.
- `r.Context()` replaces `context.Background()` so client cancellations propagate.
- `Content-Type` is set before `WriteHeader` (HTTP requires that order).
- `slog.ErrorContext(ctx, "healthcheck failed", "error", err)` is the single error-logging boundary.
- A `default:` case is added for safety, mapping unexpected errors to a generic body string. This addresses a flag from the PR 1 review.

- [ ] **Step 7.3: Strip the per-stage error slogs from `check.go`**

In `/data/git/private/mariadb-healthcheck/internal/mariadb/check.go`, remove the four `slog.ErrorContext` calls that wrap stage failures. The Scan path doesn't currently log; leave it. Keep the four `slog.Debug` "Executed query …" lines and the structure of error wrapping. The full file becomes:

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

	if deleteRow {
		if err := DeleteRow(ctx, db, uuid); err != nil {
			return fmt.Errorf("%w: %v", ErrDelete, err)
		}

		slog.Debug( //nolint:G706 // UUID has fixed format
			"Executed query to delete row",
			"UUID", uuid,
		)
	}

	_ = value // value is read by Scan; declared but otherwise unused after dropping the mismatch check.

	return nil
}
```

Note: Go doesn't complain about a written-but-not-read variable when the write is `Scan(&value)` (Scan does the write through a pointer). The `_ = value` is belt-and-braces; remove it if `go vet` is clean without it. Easiest path: keep it.

Actually, simpler: drop the `_ = value` line — `Scan(&value)` IS the use. If the linter complains, add it back.

- [ ] **Step 7.4: Update `handler_internal_test.go`**

In `/data/git/private/mariadb-healthcheck/cmd/healthcheck/handler_internal_test.go`:

1. **Remove** every `ID: uid` line from the `config{...}` literals — the field no longer exists. Drop the `uid := uuid.New()` declaration in each sub-test that only used it for the config field; the test will need a way to express the UUID that the mock expects. Since the handler now generates its own UUID per request, the mock must accept any UUID-shaped argument. Replace the specific UUID arguments in all `mock.ExpectExec(...).WithArgs(uid.String())` and `mock.ExpectQuery(...).WithArgs(uid.String())` and the row value with `sqlmock.AnyArg()`.

   For example, the `should return failed to insert row` sub-test becomes:

   ```go
   	t.Run("should return failed to insert row", func(t *testing.T) {
   		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
   		if err != nil {
   			t.Fatalf("failed to create mock database: %v", err)
   		}

   		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
   			WithArgs(sqlmock.AnyArg()).
   			WillReturnError(errors.New("insert failed"))

   		defer db.Close()

   		server := httptest.NewServer(
   			http.HandlerFunc(
   				config{
   					DBInterface: db,
   				}.healthHandler,
   			),
   		)
   		defer server.Close()

   		resp, err := http.Get(server.URL)
   		body := decodeHTTPBody(t, resp)

   		require.NoError(t, mock.ExpectationsWereMet())
   		require.NoError(t, err)
   		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
   		assert.Equal(t, "failed to insert row", body)
   	})
   ```

   For tests that need the SELECT row's value to match (so the validate path doesn't fire), use `sqlmock.AnyArg()` for arguments and use a placeholder UUID for the rows that the test doesn't try to compare. Actually — the value check is gone now; rows just need to exist. Use:

   ```go
   	mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
   		WithArgs(sqlmock.AnyArg()).
   		WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow("any-uuid"))
   ```

2. **Remove** the `should return failed to validate row` sub-test. The mismatch path is gone in PR 2; the only way to hit `ErrValidate` now is via empty-rows (sql.ErrNoRows). If you want a regression test for that path, add it explicitly:

   ```go
   	t.Run("should return failed to validate row when SELECT returns no rows", func(t *testing.T) {
   		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
   		if err != nil {
   			t.Fatalf("failed to create mock database: %v", err)
   		}

   		defer db.Close()
   		mock.ExpectExec("INSERT INTO status (uuid) VALUES (?)").
   			WithArgs(sqlmock.AnyArg()).
   			WillReturnResult(sqlmock.NewResult(1, 1))

   		mock.ExpectQuery("SELECT uuid FROM status WHERE uuid = ?").
   			WithArgs(sqlmock.AnyArg()).
   			WillReturnRows(sqlmock.NewRows([]string{"uuid"}))

   		server := httptest.NewServer(
   			http.HandlerFunc(
   				config{
   					DBInterface: db,
   				}.healthHandler,
   			),
   		)
   		defer server.Close()

   		resp, err := http.Get(server.URL)
   		body := decodeHTTPBody(t, resp)

   		require.NoError(t, mock.ExpectationsWereMet())
   		require.NoError(t, err)
   		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
   		assert.Equal(t, "failed to validate row", body)
   	})
   ```

3. **Remove** the `uuid "github.com/google/uuid"` import — no longer used by the test file.

4. **Add Content-Type assertion** to one happy-path sub-test (e.g., `should return OK, when clean table is true`):

   ```go
   	assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
   ```

- [ ] **Step 7.5: Run handler tests**

```bash
cd /data/git/private/mariadb-healthcheck
go test ./cmd/healthcheck -v -run TestHealthHandler
```

Expected: all sub-tests PASS (count went from 7 to 7: removed `validate row` mismatch, added `validate row when SELECT returns no rows`).

- [ ] **Step 7.6: Run full suite + lint**

```bash
go test ./...
golangci-lint run ./...
```

Expected: PASS, 0 issues.

- [ ] **Step 7.7: Commit**

```bash
git add cmd/healthcheck/types.go cmd/healthcheck/handler.go \
        cmd/healthcheck/handler_internal_test.go internal/mariadb/check.go
git commit -m "$(cat <<'EOF'
refactor(handler,check): drop dead config.ID; single error-log boundary

config.ID was dead code (value-receiver handler made the Nil check
unreachable). Always generate a fresh UUID per request. Move every
per-stage slog.ErrorContext out of RunCheck so the handler is the only
place errors are logged — preventing future double-logging when callers
also log returned errors. Add a default arm to the sentinel switch and
set Content-Type: text/plain; charset=utf-8 on responses. Use the
incoming request's context as the parent of the timeout context.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Final verification

- [ ] **Step 8.1: Full test, lint, race, build**

```bash
cd /data/git/private/mariadb-healthcheck
go test ./... && \
  go test -race ./... && \
  golangci-lint run ./... && \
  go build -o /tmp/healthcheck-pr2 ./cmd/healthcheck && \
  ls -la /tmp/healthcheck-pr2
```

Expected:
- All tests PASS without and with `-race`.
- golangci-lint exits 0.
- Binary `/tmp/healthcheck-pr2` produced.

- [ ] **Step 8.2: Manual smoke check for the password requirement**

Run:
```bash
DB_PASSWORD= /tmp/healthcheck-pr2 2>&1 | head -5
```

Expected output (within the first few lines): an error mentioning `DB_PASSWORD environment variable is required` and a non-zero exit code.

```bash
echo $?
```
Expected: `1`.

- [ ] **Step 8.3: Verify commit log**

Run:
```bash
git log --oneline pr/1-refactor-single-module..HEAD
```

Expected: 7 commits in this order (newest first):
```
<sha> refactor(handler,check): drop dead config.ID; single error-log boundary
<sha> fix(check): treat sql.ErrNoRows as ErrValidate; drop unreachable check
<sha> refactor(factor): replace per-field branches with envOr/envInt/envBool
<sha> feat(security): require DB_PASSWORD; drop silent default
<sha> feat(server): add graceful shutdown on SIGINT/SIGTERM
<sha> feat(server): add ReadHeaderTimeout and IdleTimeout
<sha> refactor(mariadb): build DSN via mysql.Config; tune pool; drop url.Parse
```

- [ ] **Step 8.4: Cleanup**

```bash
rm /tmp/healthcheck-pr2
```

- [ ] **Step 8.5: Stop and report**

The branch `pr/2-security-logic` is ready for PR creation. Report back with:
- Branch name.
- Commit SHAs (7 of them).
- Output of `go test ./...`, `go test -race ./...`, `golangci-lint run ./...`.
- Output of the manual smoke check (`DB_PASSWORD=` invocation).

PR creation/push is user-driven — do **not** push or open a PR without explicit instruction.

---

## Notes on behavior preservation

PR 2 introduces one deliberate user-visible breaking change: `DB_PASSWORD` is now required. Document it prominently in the PR description so anyone running on the default password understands why their next deploy fails.

Other observable changes that are not breaking but are worth noting:

1. **Log message at the boundary changed.** Previously each failed stage emitted `"failed to insert row"` (etc.) as the slog `msg` field via `slog.ErrorContext(ctx, ..., "error", err)` from inside the handler. Now the boundary message is `"healthcheck failed"` with the wrapped sentinel error in the `error` field. The semantic content is identical — just located differently in the log line. Operators relying on grepping the *msg* string will see `"healthcheck failed"` instead.
2. **Several debug logs from `parseEnv` are gone.** The "using default ..." messages are dropped; they were noise. `slog.Warn("delete row is disabled")` is preserved.
3. **Connection-pool size is now bounded at 2.** Previously `*sql.DB` was unbounded, which could leak connections under unusual probe configurations. The bound is generous for a sidecar serving one probe at a time.
4. **`Content-Type: text/plain; charset=utf-8`** is now set on responses. Probes don't read it, but it improves `kubectl describe pod` output and curl output.
5. **`r.Context()` replaces `context.Background()`** as the parent for the per-request timeout context. If the probe disconnects mid-check, the SQL operations get cancelled, freeing the connection sooner.
