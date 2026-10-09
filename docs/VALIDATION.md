# Validation evidence

Initial delivery checked on 2026-10-09 using Go 1.25.9 on Linux amd64.

## Local checks

- `make check`: gofmt, go vet, race-enabled tests, all-package build passed.
- `go test -count=1 -coverpkg=./... -coverprofile=coverage.out ./...`:
  aggregate statement coverage 84.4% (including CLI and transport packages).
- `goreleaser check` with GoReleaser v2.18.2 passed. The downloaded binary's
  SHA256 was verified against the GoReleaser release checksums.
- Cobra help/version and command registration checked from built `bin/zenodo`.
- Cross-builds for macOS arm64 and Windows amd64 passed; Linux amd64 is the
  native build. CI runs format/vet/race tests/build on all three operating systems.
  Source checkout is forced to LF through .gitattributes so Windows Git's CRLF
  conversion cannot cause false gofmt failures.
- [GitHub CI run 37866367961](https://github.com/kehao95/zenodo-cli/actions/runs/37866367961)
  passed on Linux, macOS and Windows for implementation/CI commit `28936f7`.
  Each platform passed formatting, vet, race-enabled tests and build.

Meaningful tests exercise actual command dispatch, request method/path/body,
metadata precision and unknown fields, environment auth and token redaction,
all lifecycle actions, new-version draft resolution, metadata-preserving DOI
reservation, native preview URLs, page aggregation and bounds, repeated page
detection, input rejection, immutable read-only environment policy, matching
confirmation IDs (including generic API), error codes and token redaction,
GET-only retries, cancellation, rejected redirects, streaming uploads,
collision protection, verified MD5/SHA256 downloads, and destination preservation
after checksum failure. Export is raw text; ordinary resource stdout is JSON.

## Real Zenodo checks

The existing ZENODO_ACCESS_TOKEN from the user's shell was used under
ZENODO_CLI_READ_ONLY=true. It was neither printed nor saved to the repository.

| Check | Result |
| --- | --- |
| auth test | Authenticated GET /deposit/depositions?size=1 passed |
| records search --query langton --size 1 | Native search JSON returned; one record retrieved |
| records download 4618441 treatment.html | 1143-byte public file; MD5 matched 13449cad6358f42245d3f16045c3e853 |
| records export 4618441 --format bibtex | Native BibTeX returned |
| licenses get cc-by-4.0 | Current vocabulary lookup returned 200 |
| licenses list --size 1 | Current vocabulary list returned hits and pagination |

No real draft, record, DOI reservation, upload, edit, delete or publication was
created during validation. Real write behavior is covered by HTTP fixtures,
not claimed as a live server integration result. An authenticated browser preview
was not opened because no new real draft was created. Preview URL behavior was
checked against current InvenioRDM source and command fixtures.

Two live compatibility findings were fixed before delivery: file-content GET
rejects Accept: application/octet-stream, so downloads now send */*; the old
/api/licenses endpoint returns 404, so license commands use
/api/vocabularies/licenses. The latter returns current vocabulary payloads.

## Initial release

[v0.1.0](https://github.com/kehao95/zenodo-cli/releases/tag/v0.1.0) was published
on 2026-10-09 from annotated tag `v0.1.0`, commit
`c123a05781923d40a9239eb7094d7b41e0203842`.

- [Release workflow](https://github.com/kehao95/zenodo-cli/actions/runs/37866662931)
  passed and uploaded Linux, macOS and Windows archives for amd64 and arm64,
  plus `checksums.txt`.
- All six downloaded archives matched their SHA256 entries in `checksums.txt`.
- The released Linux amd64 binary reported version `0.1.0` and the tagged
  commit; metadata template generation and validation passed.
- [Tagged-source CI](https://github.com/kehao95/zenodo-cli/actions/runs/37866662928)
  passed formatting, vet, race-enabled tests and build on all three platforms.
- GitHub reports the release as published, neither draft nor prerelease.

## Simplified authentication follow-up

Current source uses only `ZENODO_ACCESS_TOKEN`. Config-file reads/writes,
`--config`, `ZENODO_CLI_CONFIG`, `auth login`/`logout`, `--sandbox`, and
endpoint-specific token selection are removed. Existing v0.1.0 release artifacts
remain unchanged. Tests cover status token redaction, missing credentials without
network access, ignored legacy config and token variables, removed command/flag
rejection, and explicit endpoint override precedence. Formatting, vet, race tests
and build are rerun for this change.

## Workspace closure

Personal repo navigation and workspace STATUS now route to this repository;
the workspace ignore list names the nested git boundary. No parent frontier
was added because the implementation is closed and the repository owns its
continuation context. The workspace root currently has no .git directory, so
workspace routing edits cannot be committed there in this environment.

The global frontier checker was run. It reports existing errors in the
ai-safety-fellowship and Shark/cloudagent/archive owner surfaces (oversized or
duplicate anchors/evidence) and expired leases. Those unrelated surfaces were
not changed. This does not affect the Zenodo repository's checks.

The cancelled full-server investigation left no running/stopped test containers,
compose network, test service volumes, local source checkout or Python virtualenv
in this repository. Routine Docker/download caches may remain on the host.
