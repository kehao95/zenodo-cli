# zenodo-cli

Zenodo for agents. A Go CLI inspired by [slk](https://github.com/kehao95/slack-agent-cli):
JSON by default, noninteractive commands, token environment variables, and
stable exit codes. Use `--human` for indented JSON.

Prepare a real Zenodo draft, inspect it in the browser, and publish explicitly.
Creating drafts and uploading files never publishes them.

## Install

Requires Go 1.25 or newer:

```sh
go install github.com/kehao95/zenodo-cli@latest
```

Or download a prebuilt **zenodo** binary from
[v0.1.0](https://github.com/kehao95/zenodo-cli/releases/tag/v0.1.0).
Archives cover Linux, macOS and Windows on amd64 and arm64, with SHA256 checksums.

From a checkout:

```sh
go build -o bin/zenodo .
./bin/zenodo --help
```

`go install` names the executable **zenodo-cli** after the module; source builds
and the configured release archives name it **zenodo**. Both accept the same
commands. Examples below use `zenodo`; substitute `zenodo-cli` for a Go install.
Tagged releases are built with GoReleaser.

## Authentication

```sh
# Use ZENODO_ACCESS_TOKEN already exported in your shell.
zenodo auth status
zenodo auth test

# Optionally persist your existing environment token; stdout never includes it.
printf '%s' "$ZENODO_ACCESS_TOKEN" | zenodo auth login --token-stdin --verify
```

Config defaults to `$XDG_CONFIG_HOME/zenodo-cli/config.json` or
`~/.config/zenodo-cli/config.json`, saved atomically. On macOS/Linux it has mode
`0600`; Windows uses inherited filesystem ACLs. Tokens are
stored per API endpoint. `--config` overrides `ZENODO_CLI_CONFIG`.

Production uses `ZENODO_ACCESS_TOKEN`; `--sandbox` uses the **separate**
`ZENODO_SANDBOX_ACCESS_TOKEN`. A production token is never automatically sent to
sandbox or a custom `--base-url`. `auth logout` removes only the selected saved
token; exported environment tokens continue to take precedence.

## Draft → preview → publish

```sh
# Generate and edit a complete metadata object.
zenodo metadata template > metadata.json
zenodo metadata validate --metadata @metadata.json

# Create a draft and capture its ID.
draft=$(zenodo drafts create --metadata @metadata.json | jq -r .id)

# Upload files without publishing.
zenodo files upload "$draft" ./dataset.zip
zenodo files list "$draft"

# Return current draft, common metadata checks, and logged-in browser URLs.
zenodo drafts preview "$draft"

# Make it public only when ready. --confirm must match the target ID.
zenodo drafts publish "$draft" --confirm "$draft"
```

Preview uses GET only. Open `preview_url` while logged into the matching Zenodo
account to see the native record preview. Local metadata checks are advisory;
Zenodo supplies full validation when saving/publishing. There is no CLI dry-run
or local Zenodo environment.

Metadata can be an object, `{"metadata": {...}}`, `@file`, or `-` for stdin.
`drafts update ID --metadata @metadata.json` replaces the complete metadata
object; retain fields you want to keep. `drafts reserve-doi ID` preserves current
metadata while setting `prereserve_doi`; reservation does not register the DOI.

## Read and download

```sh
zenodo records search --query 'climate' --size 10
zenodo records get 4618441
zenodo records files 4618441
zenodo records download 4618441 treatment.html --output ./treatment.html
zenodo records export 4618441 --format bibtex
zenodo drafts list --status draft
zenodo records search --query 'climate' --all --max-pages 5
zenodo licenses get cc-by-4.0
```

Single-page commands preserve native Zenodo JSON. `--all` returns
`{"pages": [...], "complete": true}`; a bounded result includes
`"complete": false` and a `next` URL. Maximum 100 pages by default. Exports write
raw citation/XML text to stdout. Downloads stream into a temporary file,
verify the server checksum, then install it atomically. Existing destinations
require `--force`. Upload name collisions require `--replace`.

## Editing and versions

```sh
# Edit published metadata, then preview and republish explicitly.
zenodo drafts edit RECORD_ID
zenodo drafts update RECORD_ID --metadata @metadata.json
zenodo drafts preview RECORD_ID
zenodo drafts publish RECORD_ID --confirm RECORD_ID

# A new version returns both source and the new draft (draft.id is the new ID).
zenodo drafts new-version LATEST_VERSION_ID

# Destructive commands name their target twice.
zenodo drafts discard ID --confirm ID
zenodo files delete ID FILE_ID --confirm ID
zenodo drafts delete ID --confirm ID
```

Published records cannot be deleted through the legacy deposition DELETE API.
Zenodo's current UI may have different owner deletion/file editing policies;
the CLI follows this API's permissions and reports server errors.

## Generic API and automation

```sh
zenodo api GET /records --param 'q=climate' --param size=5
zenodo api GET /communities --param size=5
zenodo api PUT /deposit/depositions/ID --data @request.json

# Optional environment lock; flags cannot override it.
ZENODO_CLI_READ_ONLY=true zenodo drafts list
```

All API paths are relative to `/api`; absolute links must share the selected
origin. Generic calls follow the same write confirmation policy. The CLI never
automatically retries mutations. GET 429/503 responses get bounded retries,
honoring `Retry-After`; use `--retries 0` to disable them. Increase `--timeout`
for large transfers (default `2m`). An interrupted write can have succeeded on
the server; inspect the draft before repeating it.

Errors are JSON on stderr, with no success output on stdout:

```json
{"error":{"exit_code":7,"code":"not_found","message":"Not found","status":404}}
```

Exit codes: `0` success, `1` general/API conflict, `2` input/configuration,
`3` authentication, `4` rate limit, `5` network, `6` permission/policy,
`7` not found.

## Development and current state

```sh
go fmt ./...
go vet ./...
go test -race ./...
go build ./...
```

The initial v0.1.0 release covers the documented deposition lifecycle and file
operations, public record lookup/export/download, licenses, and generic JSON
API calls. Automated write tests use isolated HTTP fixtures. Real credential
checks are GET-only. Implementation ownership is this repository.

- [Design and API coverage](docs/DESIGN.md)
- [Implementation plan](docs/PLAN.md)
- [Validation evidence](docs/VALIDATION.md)
- [Zenodo REST API reference](https://developers.zenodo.org/)
- [Zenodo draft and preview guide](https://help.zenodo.org/docs/deposit/create-new-upload/)

## Open

No required implementation work remains for the initial scope. Validating real
write workflows on a disposable sandbox account is an optional follow-up;
production write behavior has been tested with HTTP fixtures rather than real
account mutations.
