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
