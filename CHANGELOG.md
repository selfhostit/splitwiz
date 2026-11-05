# Changelog

What changed in each release of Splitwiz. Installs update to the newest release; see
[Updating](README.md#updating).

## Unreleased

- Runs as a normal user (1000:1000, or `PUID`/`PGID`): it starts as root only to give the
  data folder to that user, then switches. An install that keeps an older compose file keeps
  running as root and says so in its log; the installer brings the new compose file.
- The container's filesystem is read-only apart from `pb_data` and `/tmp`.
- SECURITY.md: how to report a problem privately, and how quickly fixes ship.
- Security scans on every pull request and every Monday: CodeQL, Grype (dependencies and the
  released image), dependency review and gitleaks.

## 1.0.0 (2026-11-04)

- Releases: a `vX.Y.Z` tag publishes `:X.Y.Z`, `:X.Y`, `:X` and `:latest`, and installs follow
  `:latest`, so they only update to a release. `main` publishes `:edge`.
- Every published image has a signed provenance attestation (`gh attestation verify`).
- Security and maintenance section in the README.
