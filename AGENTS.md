# Zenodo CLI

Read README.md and docs/DESIGN.md before changes. Go + Cobra, machine-first
JSON stdout; errors on stderr with stable exit codes. Canonical implementation
and plan live here, not in workspace indexes.

Run `go fmt ./...`, `go vet ./...`, `go test -race ./...`, and `go build ./...`.
Tests must use httptest and temporary files by default. Live tests are opt-in,
GET-only, with ZENODO_CLI_READ_ONLY=true. Never create, publish, reserve a DOI,
delete, edit, or upload anything on a real account as part of automated testing.
Authentication uses only ZENODO_ACCESS_TOKEN; no config files or sandbox mode.
Never log tokens or Authorization.

Enforce safety at the HTTP boundary, including generic API and redirects.
Explicit commands perform mutations; publish/delete/discard need --confirm ID.
Never retry mutations, even after a timeout.
Update docs and meaningful tests when command or safety contracts change.
