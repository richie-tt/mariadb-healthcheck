# PR 3: CI + Docker + Dependabot Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Modernise the CI/CD layer of `mariadb-healthcheck`: broaden Dependabot coverage, run a single golangci-lint and `go test ./...` job at the repo root, add `govulncheck`, build multi-arch (linux/amd64 + linux/arm64) docker images, gate auto-tagging on green CI, and pin the Dockerfile base image by digest so reproducibility no longer depends on stale apk version pins.

**Architecture:** `.github/dependabot.yml` gains two ecosystems (`github-actions`, `docker`) alongside the existing `gomod`. `.github/workflows/build.yaml` collapses the per-package test invocations into one `go test ./...`, adds `govulncheck`, sets up QEMU + multi-arch buildx, and triggers on `push` to `master` so a green master can in turn unlock auto-tagging. `.github/workflows/release-tagging.yml` switches its trigger from `push` to `workflow_run` so a tag is only minted after the build workflow concludes successfully. `Dockerfile` pins the `golang:1.25-alpine3.23` base by SHA256 digest and drops the exact apk version pins (which would otherwise break silently when alpine 3.23 retires those versions).

**Tech Stack:** GitHub Actions, golangci-lint v2.8.0, govulncheck (latest), `docker/setup-qemu-action@v3`, `docker/setup-buildx-action@v3`, `docker/build-push-action@v6`, `docker/login-action@v3`, `docker/metadata-action@v5`, `actions/checkout@v4`, `actions/setup-go@v5`, `codecov/codecov-action@v5`, Alpine 3.23.

**Spec:** `docs/superpowers/specs/2026-04-27-mariadb-healthcheck-hardening-design.md` — see PR 3 section.

**Branch:** `pr/3-ci-docker-dependabot` (already created from PR 2 HEAD `4b5c057`). When PR 2 merges to master, rebase this branch onto the new master before opening the PR.

---

## File structure (changes by file)

| File | Concerns | Changes |
| --- | --- | --- |
| `.github/dependabot.yml` | Dependabot ecosystem coverage | W1 (add github-actions + docker), W2 (single root directory) |
| `.github/workflows/build.yaml` | CI lint/test/build | W3 (single `go test ./...`), W4 (govulncheck), W5 (multi-arch buildx + setup-qemu, bump build-push to v6), trigger on `push: branches: [master]` |
| `.github/workflows/release-tagging.yml` | Auto-tagging | W6 (switch from `push` to `workflow_run`, gate on `success`) |
| `Dockerfile` | Build image | W7 (pin builder by `@sha256:…`), W8 (drop apk version pins) |

---

## Task 0: Confirm branch state baseline

**Files:** none.

- [ ] **Step 0.1: Confirm branch + clean tree**

```bash
cd /data/git/private/mariadb-healthcheck
git branch --show-current
git log --oneline master..HEAD | head -1
git status --short
```

Expected:
- Branch is `pr/3-ci-docker-dependabot`.
- The most recent commit is `4b5c057 docs: align README + SQL example with PR 2 behavior changes` (the last PR 2 commit, since PR 3 was branched from PR 2 HEAD).
- Working tree clean.

If the branch doesn't exist, create it from PR 2:
```bash
git checkout pr/2-security-logic
git checkout -b pr/3-ci-docker-dependabot
```

- [ ] **Step 0.2: Tests + lint baseline**

```bash
cd /data/git/private/mariadb-healthcheck
go test ./...
golangci-lint run ./...
```

Expected: both packages PASS, lint reports `0 issues.`

---

## Task 1: Dependabot — add `github-actions` + `docker`, single root directory (W1 + W2)

**Files:**
- Modify: `.github/dependabot.yml`

- [ ] **Step 1.1: Replace the existing dependabot config**

Overwrite `/data/git/private/mariadb-healthcheck/.github/dependabot.yml` with:

```yaml
# To get started with Dependabot version updates, you'll need to specify which
# package ecosystems to update and where the package manifests are located.
# Please see the documentation for all configuration options:
# https://docs.github.com/code-security/dependabot/dependabot-version-updates/configuration-options-for-the-dependabot.yml-file

version: 2
updates:
  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "weekly"

  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "weekly"

  - package-ecosystem: "docker"
    directory: "/"
    schedule:
      interval: "weekly"
```

Notes:
- The previous config had two `gomod` entries (`/cmd/healthcheck` and `/internal/mariadb`) — those directories no longer have their own `go.mod` files since PR 1 collapsed to a single root module. After PR 3, Dependabot tracks the root module only.
- `github-actions` ecosystem keeps the action versions in `.github/workflows/*.yml` current (`actions/checkout`, `docker/build-push-action`, etc.).
- `docker` ecosystem keeps the `FROM` line in `Dockerfile` current — including the digest pin we add in Task 6.

- [ ] **Step 1.2: Validate YAML syntax**

Run:
```bash
cd /data/git/private/mariadb-healthcheck
python3 -c "import yaml; yaml.safe_load(open('.github/dependabot.yml'))" && echo OK
```

Expected: `OK`. If `python3` isn't available, any YAML linter works; alternatively just visually confirm the file matches the block above.

- [ ] **Step 1.3: Commit**

```bash
cd /data/git/private/mariadb-healthcheck
git add .github/dependabot.yml
git commit -m "$(cat <<'EOF'
chore(dependabot): track github-actions and docker; single root for gomod

PR 1 collapsed to a single Go module so the per-submodule gomod
entries no longer apply. Replace them with a single root entry, and
add github-actions and docker ecosystems so action versions and the
Dockerfile base image stay current automatically.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Build workflow — single `go test ./...`, trigger on master (W3)

**Files:**
- Modify: `.github/workflows/build.yaml`

**Goal:** Collapse the four per-package `go test` / coverage invocations into one `go test ./... -coverprofile=coverage.txt`. Drop the manual coverage merge. Add `push: branches: [master]` to the workflow's `on:` so the build runs on every master push (prerequisite for Task 5's `workflow_run` gate).

The lint job is already a single `golangci-lint` invocation at the repo root from PR 1's CI commit — leave it alone here. The `build` job (docker push) is also untouched; Tasks 3 and 4 modify it.

- [ ] **Step 2.1: Update the `on:` block + the test job**

Open `/data/git/private/mariadb-healthcheck/.github/workflows/build.yaml` and apply two edits:

(a) Replace the `on:` block at the top of the file:

```yaml
on:
  pull_request:
    types:
      - opened
      - reopened
      - synchronize
  push:
    tags:
      - "v*"
  workflow_dispatch: {}
```

with:

```yaml
on:
  pull_request:
    types:
      - opened
      - reopened
      - synchronize
  push:
    branches:
      - master
    tags:
      - "v*"
  workflow_dispatch: {}
```

(b) Replace the entire `test:` job (currently 6 steps for per-package test+coverage+merge) with the single-invocation form:

```yaml
  test:
    name: Test and coverage
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: 1.25.x

      - name: Run tests with coverage
        run: go test -coverprofile=coverage.txt ./...

      - name: Upload results to Codecov
        uses: codecov/codecov-action@v5
        with:
          token: ${{ secrets.CODECOV_TOKEN }}
```

Do not modify the `lint:` job or the `build:` job in this step — they are addressed by other tasks.

- [ ] **Step 2.2: Validate YAML syntax**

```bash
cd /data/git/private/mariadb-healthcheck
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/build.yaml'))" && echo OK
```

Expected: `OK`.

- [ ] **Step 2.3: Run the equivalent commands locally to confirm the new shape works**

```bash
cd /data/git/private/mariadb-healthcheck
go test -coverprofile=coverage.txt ./...
ls -la coverage.txt
head -1 coverage.txt
rm coverage.txt
```

Expected:
- Both packages PASS.
- `coverage.txt` is created.
- The first line is `mode: set`.

- [ ] **Step 2.4: Commit**

```bash
cd /data/git/private/mariadb-healthcheck
git add .github/workflows/build.yaml
git commit -m "$(cat <<'EOF'
ci: collapse per-package test+coverage into a single go test ./...

Replace four per-package test/coverage steps and the manual coverage
merge with one go test -coverprofile=coverage.txt ./... invocation.
Also trigger the workflow on push to master so the upcoming
workflow_run gate on release-tagging has something to chain off of.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Add `govulncheck` step (W4)

**Files:**
- Modify: `.github/workflows/build.yaml`

**Goal:** Add a `govulncheck` invocation to the `lint` job. It runs after the existing `golangci-lint` step and fails the job on known CVEs in the dependency graph (independent of `gosec`'s rules in golangci-lint).

- [ ] **Step 3.1: Append two steps to the `lint` job**

In the `lint:` job of `/data/git/private/mariadb-healthcheck/.github/workflows/build.yaml`, after the existing `Run golangci-lint` step, add:

```yaml
      - name: Install govulncheck
        run: go install golang.org/x/vuln/cmd/govulncheck@latest

      - name: Run govulncheck
        run: govulncheck ./...
```

The complete `lint:` job should now look like:

```yaml
  lint:
    name: Lint
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: 1.25.x

      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v9.2.0
        with:
          version: v2.8.0

      - name: Install govulncheck
        run: go install golang.org/x/vuln/cmd/govulncheck@latest

      - name: Run govulncheck
        run: govulncheck ./...
```

- [ ] **Step 3.2: Run `govulncheck` locally to confirm the dep tree is clean**

```bash
cd /data/git/private/mariadb-healthcheck
go install golang.org/x/vuln/cmd/govulncheck@latest
$(go env GOPATH)/bin/govulncheck ./...
```

Expected: a "No vulnerabilities found." line at the bottom of the output. If govulncheck finds something, **stop** and report — the PR may need to bump a dependency before merge. Do NOT silence vulnerabilities.

If the binary isn't on PATH, use `$(go env GOPATH)/bin/govulncheck`. The CI runner uses `actions/setup-go` which puts `$GOPATH/bin` on PATH automatically.

- [ ] **Step 3.3: Validate YAML**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/build.yaml'))" && echo OK
```

Expected: `OK`.

- [ ] **Step 3.4: Commit**

```bash
cd /data/git/private/mariadb-healthcheck
git add .github/workflows/build.yaml
git commit -m "$(cat <<'EOF'
ci: add govulncheck step

Run govulncheck after golangci-lint in the lint job to fail CI on
known CVEs in the dependency graph. Complements gosec (which is part
of the golangci-lint set) by checking against the Go vulnerability
database directly.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Multi-arch docker build (W5)

**Files:**
- Modify: `.github/workflows/build.yaml`

**Goal:** Build the docker image for both `linux/amd64` and `linux/arm64`. Add `docker/setup-qemu-action@v3` so the runner can cross-build under emulation, and bump `docker/build-push-action` from `@v5` to `@v6` (current major).

- [ ] **Step 4.1: Update the `build` job**

Replace the entire `build:` job in `/data/git/private/mariadb-healthcheck/.github/workflows/build.yaml` with:

```yaml
  build:
    name: Build and push docker image
    runs-on: ubuntu-latest
    if: startsWith(github.ref, 'refs/tags/')
    needs:
      - lint
      - test
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3
        with:
          platforms: linux/amd64,linux/arm64

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}

      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ vars.DOCKER_HUB }}
          tags: |
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}

      - name: Build and push
        uses: docker/build-push-action@v6
        with:
          context: .
          push: true
          platforms: linux/amd64,linux/arm64
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
```

Three changes vs. the previous block:
- New `Set up QEMU` step before `Set up Docker Buildx`.
- `docker/build-push-action@v5` → `@v6`.
- `platforms: linux/amd64,linux/arm64` added to the `build-push-action` step.

The `if: startsWith(github.ref, 'refs/tags/')` gate is preserved — multi-arch builds only run on tag pushes, not on every PR.

- [ ] **Step 4.2: Validate YAML**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/build.yaml'))" && echo OK
```

Expected: `OK`.

- [ ] **Step 4.3 (optional, run if `docker buildx` is available locally): Local multi-arch dry-run**

```bash
cd /data/git/private/mariadb-healthcheck
docker buildx ls 2>&1 | head -5
```

If buildx is available:
```bash
docker buildx build --platform linux/amd64,linux/arm64 --tag healthcheck:pr3-test .
```

Expected: both architectures build successfully. If buildx isn't installed locally, skip this step — CI will be the source of truth.

- [ ] **Step 4.4: Commit**

```bash
cd /data/git/private/mariadb-healthcheck
git add .github/workflows/build.yaml
git commit -m "$(cat <<'EOF'
ci: build multi-arch docker images (linux/amd64, linux/arm64)

Add docker/setup-qemu-action so the runner can cross-build under
emulation, set platforms on docker/build-push-action, and bump
build-push-action @v5 -> @v6 (current major). Builds still only run
on tag pushes, so the doubled build time only affects releases.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Gate release-tagging on green CI (W6)

**Files:**
- Modify: `.github/workflows/release-tagging.yml`

**Goal:** Change the auto-tagger trigger from `push: branches: [master]` to `workflow_run` on the build workflow's success, so a red master can no longer be auto-tagged. The existing tag-bumping action (`rymndhng/release-on-push-action`) stays the same.

- [ ] **Step 5.1: Replace the `on:` and `jobs:` blocks**

Overwrite `/data/git/private/mariadb-healthcheck/.github/workflows/release-tagging.yml` with:

```yaml
name: Release tagging

on:
  workflow_run:
    workflows: ["Lint, Test and Build"]
    types: [completed]
    branches: [master]
  workflow_dispatch: {}

defaults:
  run:
    shell: bash

permissions:
  contents: write
  pull-requests: read

jobs:
  release-on-push:
    runs-on: ubuntu-latest
    if: ${{ github.event_name == 'workflow_dispatch' || github.event.workflow_run.conclusion == 'success' }}
    env:
      GITHUB_TOKEN: ${{ secrets.GH_TOKEN }}
    steps:
      - name: Tag/Release on Push Action
        uses: rymndhng/release-on-push-action@v0.28.0
        with:
          tag_prefix: "v"
          bump_version_scheme: patch
```

Three behaviour changes:
- **Trigger:** `workflow_run` on `Lint, Test and Build` succeeding on `master`. (The workflow name string must match the `name:` field in `build.yaml`, which is `Lint, Test and Build`.)
- **Gate:** `if: github.event.workflow_run.conclusion == 'success'` (with a special case to still allow `workflow_dispatch` so a maintainer can force a tag manually).
- **`workflow_dispatch`:** kept so the auto-tagger can still be invoked manually.

- [ ] **Step 5.2: Validate YAML**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release-tagging.yml'))" && echo OK
```

Expected: `OK`.

- [ ] **Step 5.3: Commit**

```bash
cd /data/git/private/mariadb-healthcheck
git add .github/workflows/release-tagging.yml
git commit -m "$(cat <<'EOF'
ci: gate auto-tagging behind successful CI run

Switch the release-tagging trigger from push:branches:master to
workflow_run on the build workflow completing successfully. A red
master push no longer auto-creates a tag. workflow_dispatch is kept
so maintainers can still force a tag manually.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Pin Dockerfile builder by digest (W7)

**Files:**
- Modify: `Dockerfile`

**Goal:** Replace `FROM golang:1.25-alpine3.23` with `FROM golang:1.25-alpine3.23@sha256:<digest>`. The digest below was captured at the time this plan was written (`docker manifest inspect golang:1.25-alpine3.23` returned the multi-arch index digest). If the implementer wants the absolutely latest digest at the moment of the commit, re-run the manifest inspect command — but the value here is acceptable.

- [ ] **Step 6.1: Capture the current digest (optional refresh)**

If you want to pin to the very latest digest right now:
```bash
docker manifest inspect golang:1.25-alpine3.23 | grep -m1 -oE 'sha256:[a-f0-9]{64}'
```

Expected: a `sha256:…` line. The plan was written with `sha256:30e1078ea1ce91dcd8f48f27c0d7549cf23b32019de8b78cc0dc7b7707987dd5`. Use whichever you prefer; Dependabot will keep it current.

- [ ] **Step 6.2: Update the Dockerfile builder stage**

In `/data/git/private/mariadb-healthcheck/Dockerfile`, change line 1:

From:
```dockerfile
FROM golang:1.25-alpine3.23 AS builder
```

To:
```dockerfile
FROM golang:1.25-alpine3.23@sha256:30e1078ea1ce91dcd8f48f27c0d7549cf23b32019de8b78cc0dc7b7707987dd5 AS builder
```

(The runner stage `FROM scratch AS runner` is left unchanged — `scratch` has no digest.)

- [ ] **Step 6.3: Verify the build still works locally**

```bash
cd /data/git/private/mariadb-healthcheck
docker build -t healthcheck:pr3-w7 . 2>&1 | tail -5
```

Expected: image builds successfully and reports the digest-pinned base. Then clean up:
```bash
docker rmi healthcheck:pr3-w7
```

If `docker` isn't available locally, skip the local build — CI will exercise it on the next tag push.

- [ ] **Step 6.4: Commit**

```bash
cd /data/git/private/mariadb-healthcheck
git add Dockerfile
git commit -m "$(cat <<'EOF'
build(docker): pin builder base image by digest

Replace mutable golang:1.25-alpine3.23 tag with the @sha256:... digest
so reproducibility doesn't depend on whatever the registry currently
serves. Dependabot's docker ecosystem will keep this up to date now
that PR 3 also wires that ecosystem in.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Drop apk version pins (W8)

**Files:**
- Modify: `Dockerfile`

**Goal:** Replace the exact apk version pins (`make=4.4.1-r3 git=2.52.0-r0`) with unpinned package names. Reproducibility now relies on the digest pin from Task 6; the apk pins gave a false sense of stability and silently break when alpine 3.23 retires those exact versions from its active repo.

- [ ] **Step 7.1: Update the `RUN apk add` line**

In `/data/git/private/mariadb-healthcheck/Dockerfile`, change:

```dockerfile
RUN apk add --no-cache \
    make=4.4.1-r3 \
    git=2.52.0-r0 \
    && make
```

to:

```dockerfile
RUN apk add --no-cache make git \
    && make
```

(Keep the trailing `&& make` step that runs the project's Makefile inside the builder.)

- [ ] **Step 7.2: Verify the build still works locally**

```bash
cd /data/git/private/mariadb-healthcheck
docker build -t healthcheck:pr3-w8 . 2>&1 | tail -10
```

Expected: image builds successfully — `make` and `git` are pulled from the alpine 3.23 repo without specific versions. Then clean up:
```bash
docker rmi healthcheck:pr3-w8
```

If `docker` isn't available locally, skip — the CI build (which runs the Dockerfile) will be the source of truth.

- [ ] **Step 7.3: Commit**

```bash
cd /data/git/private/mariadb-healthcheck
git add Dockerfile
git commit -m "$(cat <<'EOF'
build(docker): drop apk version pins

Replace make=4.4.1-r3 git=2.52.0-r0 with unpinned package names. The
exact pins gave a false sense of reproducibility — they silently
break when alpine 3.23 retires those exact versions. Real
reproducibility now comes from the @sha256: pin on the base image
(Task 6).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Final verification

- [ ] **Step 8.1: Tests + lint + govulncheck + build**

```bash
cd /data/git/private/mariadb-healthcheck
go test ./... && \
  go test -race ./... && \
  golangci-lint run ./... && \
  $(go env GOPATH)/bin/govulncheck ./... && \
  go build -o /tmp/healthcheck-pr3 ./cmd/healthcheck && \
  ls -la /tmp/healthcheck-pr3 && \
  rm /tmp/healthcheck-pr3
```

Expected:
- All Go tests PASS (with and without `-race`).
- golangci-lint exits 0.
- govulncheck reports no vulnerabilities.
- Binary builds.

- [ ] **Step 8.2: Validate all touched YAML files**

```bash
for f in .github/dependabot.yml .github/workflows/build.yaml .github/workflows/release-tagging.yml; do
  python3 -c "import yaml,sys; yaml.safe_load(open('$f'))" && echo "$f: OK"
done
```

Expected: three `OK` lines.

- [ ] **Step 8.3 (optional): Local docker build**

If `docker` is available:
```bash
cd /data/git/private/mariadb-healthcheck
docker build -t healthcheck:pr3-final .
docker run --rm -e DB_PASSWORD=test healthcheck:pr3-final 2>&1 | head -5
docker rmi healthcheck:pr3-final
```

Expected: image builds, container starts, exits with the validation error mentioning `DB_PASSWORD environment variable is required` (because we passed `DB_PASSWORD=test` but the container can't reach a real MariaDB; first `/health` will fail, but the binary itself starts cleanly past the env-parse phase, then the lazy DB connect surfaces).

Note: actual probing requires a real MariaDB; the smoke check is just "binary starts and reaches steady state without crashing on env parse".

- [ ] **Step 8.4: Verify commit log**

```bash
git log --oneline pr/2-security-logic..HEAD
```

Expected: 7 commits in this order (newest first):
```
<sha> build(docker): drop apk version pins
<sha> build(docker): pin builder base image by digest
<sha> ci: gate auto-tagging behind successful CI run
<sha> ci: build multi-arch docker images (linux/amd64, linux/arm64)
<sha> ci: add govulncheck step
<sha> ci: collapse per-package test+coverage into a single go test ./...
<sha> chore(dependabot): track github-actions and docker; single root for gomod
```

- [ ] **Step 8.5: Stop and report**

The branch `pr/3-ci-docker-dependabot` is ready for PR creation. Report back with:
- Branch name.
- Commit SHAs (7 of them).
- Output of `go test ./...`, `go test -race ./...`, `golangci-lint run ./...`, `govulncheck ./...`.
- Output of YAML validation.
- (Optional) Output of local `docker build`.

PR creation/push is user-driven — do not push or open a PR without explicit instruction.

---

## Notes

1. **Workflow name string in `release-tagging.yml`.** The `workflow_run.workflows` field must match the `name:` line of the build workflow exactly. Today that's `Lint, Test and Build`. If the build workflow's `name:` is ever changed, this gating string must be updated.
2. **`workflow_run` gotcha.** The `workflow_run` event runs in the context of the *default branch's* version of the gating workflow file. When PR 3 lands on master, the existing build-workflow run for master (still on the pre-PR-3 version) won't trigger the new tag-gate. The next master push after merge will be the first to use the gate.
3. **`build-push-action@v6`.** Bumping major versions sometimes changes input names. Verified that `context`, `push`, `platforms`, `tags`, and `labels` are unchanged between v5 and v6.
4. **Dependabot patches.** With three ecosystems on a weekly cadence, expect ~1–3 PRs per week. Consider auto-merging patch-level updates once the gating CI is green.
5. **govulncheck on day one.** The PR fails to land if govulncheck finds anything. If it does, this PR also includes the dep bumps to clear it.
