# mariadb-healthcheck — hardening, refactor, and CI tightening

**Date:** 2026-04-27
**Status:** Approved (pending spec review)
**Scope:** Three sequential PRs landing structural cleanup, security/logic fixes, and CI/Docker/Dependabot updates.

---

## Context

`mariadb-healthcheck` is a Go HTTP sidecar for MariaDB pods in Kubernetes. On each `GET /health` it executes `INSERT → SELECT → DELETE` against a dedicated `status` table and returns `200 OK` or `500`. The check runs synchronously in the HTTP handler — there is no background goroutine.

A critical deployment fact shapes this design: **MariaDB does not have a built-in healthcheck, so both the `mariadb` container and the `healthcheck` sidecar in the pod point their `livenessProbe`/`readinessProbe` at the sidecar's `/health` endpoint.** That makes the sidecar the single source of truth for pod health. If the sidecar crashes, MariaDB's liveness probe (which targets the sidecar) returns `connection-refused` and Kubernetes kills MariaDB itself. Therefore the sidecar must come up immediately, must not crash on transient DB errors, and must serve `500` (not crash) while MariaDB is still starting.

## Non-goals

- Replace the polling model with cached background checks. (Goroutine-based polling explicitly rejected.)
- Change the wire contract of `/health` (status codes, body strings stay identical).
- Rename environment variables (the README documents them as a public contract).
- Add new endpoints (`/ready`, `/metrics`, etc.) — out of scope.

## Decisions locked during brainstorming

| Topic | Decision |
| --- | --- |
| Background polling | Stay synchronous in handler. No goroutines. |
| PR strategy | Three sequential PRs: refactor → security/logic → CI/Docker. |
| Default password (`DB_PASSWORD`) | Drop the default. Require explicit value. `DB_USER` keeps its `healthcheck` default. |
| Startup connectivity | Lazy: `sql.Open` only, first `/health` surfaces real connect. Crash-loop on startup is unacceptable given the topology above. |
| Module path after collapse | `github.com/richie-tt/mariadb-healthcheck`. |
| Multi-arch docker | `linux/amd64` + `linux/arm64`. |
| Row leak on DELETE failure (L7) | Accept. With `MEMORY` engine resets on restart; with `ARIA` it is an edge case worth a README note, not code. |

---

## PR 1 — Refactor (no behavior change)

### Files touched
- `go.mod`, `go.sum` (new at repo root)
- `cmd/healthcheck/go.mod`, `cmd/healthcheck/go.sum`, `internal/mariadb/go.mod`, `internal/mariadb/go.sum`, `go.work`, `go.work.sum` (deleted)
- `cmd/healthcheck/handler.go`, `cmd/healthcheck/handler_internal_test.go` (imports + thinned)
- `cmd/healthcheck/main.go` (imports)
- `cmd/healthcheck/types.go` (imports)
- `internal/mariadb/check.go` (new)
- `internal/mariadb/check_test.go` (new)
- `internal/mariadb/queries.go` (functions instead of methods on `Query`)
- `internal/mariadb/types.go` (drop `Query`)
- `internal/mariadb/connection.go`, `internal/mariadb/connection_test.go`, `internal/mariadb/queries_test.go` (import path)
- `Makefile` (build target unchanged; verify path)
- `.golangci.bck.yml` (deleted)

### What changes

1. **Single Go module (C1).** Delete the two `go.mod`/`go.sum` pairs and `go.work*`. Create a single `go.mod` at repo root with module path `github.com/richie-tt/mariadb-healthcheck`. Update the import in `cmd/healthcheck/handler.go` (and any test files) from `"mariadb"` to `"github.com/richie-tt/mariadb-healthcheck/internal/mariadb"`.
2. **Split check from HTTP (C3).** New file `internal/mariadb/check.go` exposing `func RunCheck(ctx context.Context, db *sql.DB, id string, deleteRow bool) error`. The function runs `INSERT → SELECT → DELETE` for the given `id` and returns a wrapped error on failure. UUID generation stays in the handler for PR 1 (preserving existing behavior and test setup that passes specific UUIDs); PR 2 then moves UUID generation off the handler entirely. The handler shrinks to UUID generation + context setup + `RunCheck` call + status-code mapping. Status codes and body strings stay byte-identical.
3. **Drop `Query` indirection (C4).** Replace `Query{Value: x}.InsertRow(ctx, db)` etc. with package-level functions `mariadb.InsertRow(ctx, db, value string) error` (and `SelectRow`, `DeleteRow`). Delete `Query` from `internal/mariadb/types.go`. `Connection` stays.
4. **Delete stale lint backup (C5).** Remove `.golangci.bck.yml`.

### Tests
- Existing tests updated for the new layout. Handler tests get smaller because the SQL sequence moves out of the handler.
- New tests for `RunCheck`: happy path, INSERT fail, SELECT fail, scan fail, value mismatch (now unreachable but kept as a guard-test until PR 2 removes it), DELETE fail, `DeleteRow=false` skip path.
- Coverage stays ≥ current value reported by codecov.

### Acceptance
- `go test ./...` passes from repo root.
- `golangci-lint run ./...` passes.
- Built binary behaves identically: same env vars, same `/health` outputs for the same DB conditions.

---

## PR 2 — Security + Logic fixes

### Files touched
- `internal/mariadb/connection.go`
- `internal/mariadb/connection_test.go`
- `internal/mariadb/check.go` (from PR 1)
- `internal/mariadb/check_test.go` (new tests)
- `cmd/healthcheck/main.go`
- `cmd/healthcheck/main_test.go`
- `cmd/healthcheck/factor.go`
- `cmd/healthcheck/factor_internal_test.go`
- `cmd/healthcheck/const.go`
- `cmd/healthcheck/handler.go`
- `cmd/healthcheck/types.go`
- `README.md` (env-var table)

### Security changes

1. **S1 — DSN via `mysql.Config`.** In `connection.go`:
   ```go
   cfg := mysql.NewConfig()
   cfg.User = c.User
   cfg.Passwd = c.Password
   cfg.Net = "tcp"
   cfg.Addr = net.JoinHostPort(c.Host, c.Port)
   cfg.DBName = c.Database
   cfg.ParseTime = true
   cfg.Timeout = 5 * time.Second
   db, err := sql.Open(c.Driver, cfg.FormatDSN())
   ```
   Special characters in passwords are now safe.
2. **S2 — Require `DB_PASSWORD`.** Remove `defaultDBPassword` from `const.go`. In `parseEnv`, return `fmt.Errorf("DB_PASSWORD environment variable is required")` when empty. Keep `DB_USER` default for ergonomics.
3. **S5 — HTTP server timeouts.** Add `ReadHeaderTimeout: 5 * time.Second` and `IdleTimeout: 30 * time.Second` to `http.Server` in `setupServer`.
4. **S6 — Drop misleading `url.Parse` host check.** In `Connection.Validate`, the host validation reduces to non-empty + (optionally) `net.ParseIP` or `idna.ToASCII` later. For this PR: just non-empty. `net.JoinHostPort` is the formatter; the driver does the resolution.
5. **S7 — Connection pool tuning.** After `sql.Open`:
   ```go
   db.SetMaxOpenConns(2)
   db.SetMaxIdleConns(1)
   db.SetConnMaxLifetime(5 * time.Minute)
   db.SetConnMaxIdleTime(1 * time.Minute)
   ```
   Sized for one probe at a time. Prevents pool growth on long-lived pods.

### Logic changes

6. **L1 — Always generate UUID per request.** Delete the dead `if c.ID == uuid.Nil` block in `handler.go`. UUID generation moves into `RunCheck`. Drop the `ID uuid.UUID` field from `config` (in `types.go`); tests stop passing a pre-built `ID`.
7. **L2 — Handle `sql.ErrNoRows` distinctly.** In `RunCheck`, after `Scan`, if `errors.Is(err, sql.ErrNoRows)` return a wrapped error tagged "inserted row not found" (a real signal of storage anomalies). Remove the dead `value != id` comparison since the WHERE clause guarantees match.
8. **L3 — Single error-logging boundary.** `RunCheck` returns wrapped errors with consistent verbs (`"insert: ...", "select: ...", "scan: ...", "delete: ..."`). The handler logs once with `slog.ErrorContext` instead of N times. Body string for HTTP response stays the original (`"failed to insert row"` etc.) by branching on the wrapped error message; alternatively use sentinel errors `ErrInsert`, `ErrSelect`, etc., from the `mariadb` package.
9. **L4 — Simplify `parseEnv`.** Replace per-field `if empty default; if non-empty parse` with helpers:
   ```go
   func envOr(key, fallback string) string
   func envInt(key string, fallback int) (int, error)
   func envBool(key string, fallback bool) (bool, error)
   ```
   Collapse `getEnv` + `parseEnv` into one `loadConfig() (*config, error)`.
10. **L5 — Set `Content-Type`.** In `writeBody` (or wherever responses are written), set `Content-Type: text/plain; charset=utf-8` once at the top of the handler.
11. **L6 — Graceful shutdown.** In `run()`:
    ```go
    ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
    defer cancel()
    go func() {
        <-ctx.Done()
        shutdownCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
        defer c()
        _ = server.Shutdown(shutdownCtx)
    }()
    ```
    On `ListenAndServe`'s `http.ErrServerClosed` exit cleanly; close the DB after.

### Tests
- DSN round-trip: password containing `@:/?#` should produce a parseable DSN and be reflected back correctly via `mysql.ParseDSN`.
- `parseEnv` returns the required-password error when `DB_PASSWORD=""`.
- `RunCheck` `sql.ErrNoRows` case (mock returns no rows after insert).
- Pool config: `db.Stats().MaxOpenConnections == 2`.
- Graceful shutdown: send SIGTERM, expect `Shutdown` called and clean exit within timeout.
- Existing handler tests updated for the UUID-per-request behavior (no longer set `ID` on `config`).

### Acceptance
- All tests green. Coverage ≥ current.
- `golangci-lint` clean.
- Manual smoke: build the binary, run with `DB_PASSWORD=` (empty) → fails with clear error. Run with `DB_PASSWORD=p@ss:w/o?rd#` → starts and `/health` works against a local MariaDB.

### Risk
- **S2 is the only user-visible break.** Anyone running with the default password will see startup failure with the message `"DB_PASSWORD environment variable is required"`. PR description must call this out as a deliberate, security-motivated breaking change.

---

## PR 3 — Workflows + Dockerfile + Dependabot

### Files touched
- `.github/dependabot.yml`
- `.github/workflows/build.yaml`
- `.github/workflows/release-tagging.yml`
- `Dockerfile`

### Dependabot changes

1. **W1 + W2 — three ecosystems, single root.**
   ```yaml
   version: 2
   updates:
     - package-ecosystem: gomod
       directory: /
       schedule: { interval: weekly }
     - package-ecosystem: github-actions
       directory: /
       schedule: { interval: weekly }
     - package-ecosystem: docker
       directory: /
       schedule: { interval: weekly }
   ```

### Build workflow changes (`build.yaml`)

2. **W3 — single lint and test job.** Replace the two per-submodule `golangci-lint` invocations with one against the repo root. Replace the two per-submodule `go test`/coverage invocations with `go test ./...` and `go test -coverprofile=coverage.txt ./...`. Drop the manual coverage merge.
3. **W4 — `govulncheck`.** New step in the lint job, after lint, before tests:
   ```yaml
   - name: Install govulncheck
     run: go install golang.org/x/vuln/cmd/govulncheck@latest
   - name: Run govulncheck
     run: govulncheck ./...
   ```
   Fails the job on known CVEs.
4. **W5 — multi-arch docker.** Add `docker/setup-qemu-action@v3` before `setup-buildx`. Set `platforms: linux/amd64,linux/arm64` on the build-push step. Bump `docker/build-push-action@v5` → `@v6`.
5. **Trigger on master.** Add `push: branches: [master]` to the workflow `on:` so lint+test run on master pushes (currently only PRs and tags). This is the prerequisite for the release-tagging fix below.

### Release-tagging changes (`release-tagging.yml`)

6. **W6 — gate auto-tagging behind successful CI.** Switch from `push: branches: [master]` to:
   ```yaml
   on:
     workflow_run:
       workflows: ["Lint, Test and Build"]
       types: [completed]
       branches: [master]
   ```
   Add a job-level `if: ${{ github.event.workflow_run.conclusion == 'success' }}`. A red master no longer auto-tags.

### Dockerfile changes

7. **W7 — pin builder by digest.** Replace `FROM golang:1.25-alpine3.23` with `FROM golang:1.25-alpine3.23@sha256:<digest>`. Look up the digest at PR-creation time. Dependabot's `docker` ecosystem will keep it current. The runner stage stays `FROM scratch` (scratch has no digests).
8. **W8 — drop apk version pins.** Replace `apk add --no-cache make=4.4.1-r3 git=2.52.0-r0` with `apk add --no-cache make git`. Reproducibility relies on the digest pin from W7. Eliminates silent breakage when alpine 3.23 retires those pinned versions.

### Tests
- CI itself is the test. The PR is green when:
  - Single golangci-lint job passes.
  - Single test job passes; `coverage.txt` exists.
  - `govulncheck ./...` passes.
  - Build job (on tag) produces both `linux/amd64` and `linux/arm64` images.
- Local validation: `docker buildx build --platform linux/amd64,linux/arm64 .` succeeds.
- `govulncheck ./...` should pass against current `go.sum`. If it fails, this PR also bumps the affected dep(s).

### Acceptance
- One green check on PR 3.
- After merge, a master push triggers `Lint, Test and Build`; on success the release-tagging workflow runs and creates a patch tag. Confirm with one merge.
- Multi-arch image visible on Docker Hub for the next tag.

### Risk
- **W6 is the most behavior-changing piece.** Master pushes that previously auto-tagged will no longer tag if CI fails. That's the desired behavior, but flag it in the PR description so maintainers know.
- Multi-arch build roughly doubles the build job time. Acceptable for a release-only build.
- `govulncheck` may fail on day one if current deps have known CVEs; if so, this PR includes the dep bumps to clear it.

---

## Out of scope (deferred)

- TLS for DB connection (S4): low value for loopback sidecar; revisit if a remote-DB use case appears.
- Auto-cleanup of leaked rows on DELETE failure (L7): accept the leak; document in README for `ARIA`.
- Slowloris-grade hardening beyond `ReadHeaderTimeout`/`IdleTimeout` (rest of S5).
- New `/ready` or `/metrics` endpoints.
- Switching from `database/sql` to a different driver/abstraction.
