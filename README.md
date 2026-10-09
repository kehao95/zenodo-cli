# zenodo-cli

Zenodo for agents. A Go CLI inspired by [slk](https://github.com/kehao95/slack-agent-cli):
JSON by default, noninteractive commands, token environment variables, and
stable exit codes. Use `--human` for indented JSON.

Prepare a real Zenodo draft and inspect it before publication. Creating drafts
and uploading files never publishes them.

**Agent publication rule:** draft preparation can proceed automatically within
the requested task. Before every publish or republish, present the exact draft
ID, title, version, files and preview link, explain the public action, then obtain
a **separate explicit human confirmation**. A general “prepare/update/refresh”
request is not that confirmation. Material changes after review require renewed
confirmation. This applies to both `drafts publish` and generic API publishing.
`--confirm ID` checks the target; an agent supplying it is not human approval.
The CLI stays noninteractive and cannot technically prove who approved the action.
This rule is agent guidance; it does not add a CLI prompt, flag or preview field.

When asking for approval, include the draft ID, title, version (or state that it
is unset), file names/sizes/checksums, and preview URL. For republishing, also
summarize changes from the published version. Wait for the human's reply before
running a publish command. If the reviewed contents materially change, present
the updated draft and obtain a new confirmation.

## Install

Requires Go 1.25 or newer:

```sh
go install github.com/kehao95/zenodo-cli@latest
```

Or download a prebuilt **zenodo** binary from
[v0.1.3](https://github.com/kehao95/zenodo-cli/releases/tag/v0.1.3).
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
```

Authentication uses only `ZENODO_ACCESS_TOKEN` from your shell (for example,
`~/.zshrc`). `auth status` reports whether it is set, and `auth test` verifies
it with a GET. Unset the variable to stop using the token.

The CLI does not read or write credential files. There is no login/logout flow
or sandbox mode. Existing v0.1.0 config files are ignored and left untouched.
The API defaults to `https://zenodo.org/api`; `--base-url` or `ZENODO_BASE_URL`
can explicitly override it and will use the same token. Response links and
redirects must stay on the selected origin.

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

# STOP: show the prepared draft to the human and obtain separate publish confirmation.
# Only after that response: --confirm must match the reviewed target ID.
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

Prefer these commands over ad hoc HTTP scripts for record metadata and file
downloads; `api GET` covers read endpoints without a dedicated command.
Server checksum verification checks transfer integrity. When exact artifact
identity matters, also compare the downloaded file with the expected local file:

```sh
zenodo records download RECORD_ID dataset.zip --output ./downloaded.zip
sha256sum ./dataset.zip ./downloaded.zip  # Linux
# macOS: shasum -a 256 ./dataset.zip ./downloaded.zip
```

The two SHA256 digests must match. A successful download alone does not establish
that the published artifact matches the intended local file.

Single-page commands preserve native Zenodo JSON. `--all` returns
`{"pages": [...], "complete": true}`; a bounded result includes
`"complete": false` and a `next` URL. Maximum 100 pages by default. Exports write
raw citation/XML text to stdout. Downloads stream into a temporary file,
verify the server checksum, then install it atomically. Existing destinations
require `--force`. Upload name collisions require `--replace`.

## Editing and versions

```sh
# Edit published metadata, then preview and obtain a NEW human publish confirmation.
zenodo drafts edit RECORD_ID
zenodo drafts update RECORD_ID --metadata @metadata.json
zenodo drafts preview RECORD_ID
# STOP until the human confirms this specific republish.
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
origin. Generic calls follow the same write confirmation policy. The agent's
separate human-confirmation rule also applies to generic publishing:

```sh
# Only after the human approves the reviewed draft, as described above.
zenodo api POST "/deposit/depositions/$draft/actions/publish" --confirm "$draft"
```

The CLI never automatically retries mutations. GET 429/503 responses get bounded
retries, honoring `Retry-After`; use `--retries 0` to disable them. Increase `--timeout`
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

v0.1.3 adds agent publication and download-verification instructions.
CLI behavior is unchanged from v0.1.2, which uses one token environment variable.
The v0.1.0 release binaries retain their original config-file support.

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

[Issue #1](https://github.com/kehao95/zenodo-cli/issues/1) records the publication
approval gap. README and AGENTS.md now document separate human confirmation and
native download verification. This follow-up changes instructions only; CLI
help, preview JSON and HTTP behavior remain as released in v0.1.2. The issue's
original help/preview acceptance items are outside this narrowed scope. Human
approval provenance remains a workflow responsibility.
