# Security Policy

## Supported Versions

Only the latest released version of `mariadb-healthcheck` is supported with
security updates. Older versions will not receive backports — please upgrade to
the most recent tag before reporting an issue.

| Version  | Supported          |
| -------- | ------------------ |
| latest   | :white_check_mark: |
| < latest | :x:                |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues,
discussions, or pull requests.**

Instead, report them privately via GitHub's
[Private Vulnerability Reporting](https://github.com/richie-tt/mariadb-healthcheck/security/advisories/new)
feature. This creates a private channel between you and the maintainers.

When reporting, please include as much of the following as you can:

- A description of the issue and its potential impact
- Steps to reproduce (proof-of-concept, minimal config, affected version/commit)
- Any suggested mitigation or fix
- Whether you intend to disclose publicly, and on what timeline

You should receive an acknowledgement within **5 business days**. If the issue
is confirmed, we aim to release a fix within **30 days** of the initial report,
depending on complexity. We will keep you informed of progress and coordinate
disclosure timing with you.

## Scope

In scope:

- The `mariadb-healthcheck` source code in this repository
- The published Docker image
- GitHub Actions workflows in `.github/workflows/`

Out of scope:

- Vulnerabilities in third-party dependencies (please report upstream; we will
  pick them up via Dependabot)
- Issues that require a malicious operator with full cluster admin privileges
- Misconfiguration of the surrounding Kubernetes cluster or MariaDB instance

## Supply Chain

To reduce supply-chain risk:

- All third-party GitHub Actions are pinned to a full commit SHA (with the
  human-readable tag in a trailing comment)
- Dependabot is enabled for `gomod`, `github-actions`, and `docker` ecosystems
- `govulncheck` runs on every push and pull request
- Container images are built from a minimal base and published with
  multi-platform manifests
