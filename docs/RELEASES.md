# v0.1.3 — Agent workflow instructions

This release updates instructions only. CLI commands, help, preview JSON and
HTTP behavior are unchanged from v0.1.2.

- Prepare drafts, metadata and uploads within the requested task without
  repeated approvals.
- Before every publish or republish, show the draft ID, title, version, files
  and preview link, then obtain separate explicit human confirmation.
- General prepare/update/refresh requests do not authorize publication. Material
  changes after review require a renewed confirmation.
- `--confirm ID` checks the target only; it does not prove human approval.
  The same instruction applies to generic API publishing.
- Prefer native CLI downloads, then compare SHA256 with the expected local
  artifact when exact file identity matters.

These instructions are in README, AGENTS.md and design guidance. README and
project documentation are included in the release archives. The noninteractive
CLI does not enforce approval provenance or introduce an approval prompt/flag.

Install a release archive or run:

```sh
go install github.com/kehao95/zenodo-cli@v0.1.3
```

# v0.1.2 — Simplified authentication

Authentication uses only `ZENODO_ACCESS_TOKEN` from your shell. No credential
files, login/logout flow, sandbox mode, or additional token variables are needed.
Existing config files are ignored and left untouched.

`auth status` reports whether the token is set without revealing its value;
`auth test` verifies it with a GET. The default endpoint is Zenodo; explicit
`--base-url` / `ZENODO_BASE_URL` overrides use the same token. Response links
and redirects remain restricted to the selected origin.

Record, draft and file commands retain their existing behavior. Automated write
tests use HTTP fixtures; real credentials are never used for write tests.

Install a release archive or run:

```sh
go install github.com/kehao95/zenodo-cli@v0.1.2
```

Formatting, vet, race-enabled tests and build passed locally. Tests cover token
redaction, missing credentials without requests, ignored legacy config and token
variables, endpoint precedence, and removed command/flag rejection.

v0.1.1 was an intermediate tagged build and was not published as a release.

# v0.1.0 — Initial release

Zenodo CLI for agents and scripts, inspired by slk. JSON output by default,
noninteractive commands, and stable exit codes.

## Included

- Native draft creation, metadata updates, DOI reservation and browser preview.
- Explicit publish, metadata editing, discard, deletion and new-version workflows.
- Streamed uploads, file listing/rename/order/delete, and verified downloads.
- Public record search, metadata retrieval, citation export and license lookup.
- Endpoint-specific authentication, secure Unix config permissions, generic JSON
  REST requests and optional environment-enforced read-only mode.
- Linux, macOS and Windows binaries for amd64 and arm64, with SHA256 checksums.

## Install

Download the archive for your operating system and architecture from the release
assets, verify it against `checksums.txt`, and extract the `zenodo` executable.

Or install the pinned Go module:

```sh
go install github.com/kehao95/zenodo-cli@v0.1.0
```

Go installation names the executable `zenodo-cli`; release archives name it
`zenodo`. Both use the same commands. Existing `ZENODO_ACCESS_TOKEN` credentials
are picked up automatically on production.

## Validation and limitations

Linux, macOS and Windows CI passed formatting, vet, race-enabled tests and build.
Aggregate local test coverage is 84.4%. Authentication, search, file download,
checksum verification, BibTeX export and license lookup passed real GET-only
Zenodo checks. Writes are covered by HTTP fixtures; no production record was
created or published during testing.

This initial release uses the documented legacy deposition API alongside current
record and vocabulary endpoints. Metadata validation is advisory; Zenodo owns
full server validation. Resumable uploads, OAI-PMH harvesting and community
moderation are outside this version's scope.
