# Zenodo CLI design

## Contract

Go + Cobra, independent command instances for testing. `cmd/` adapts user input;
`internal/config/` owns credential selection; `internal/zenodo/` owns HTTP policy,
API errors and streaming transfers. No SDK, Viper, local database or persistent
agent process is required.

Standard resource commands return native JSON without normalization or lost
metadata fields. Composite results (`preview`, `newversion`, `--all`) explicitly
wrap native payloads. `--human` indents JSON. Help/version/completion and raw
record export are intentional text outputs. Error messages are JSON on stderr.
Numeric identifiers and metadata JSON numbers retain precision via json.Number.

## Commands and endpoint coverage

`drafts` is an alias for `depositions`; `new-version` aliases `newversion`.

| Command | API request |
| --- | --- |
| auth test / login --verify | GET /deposit/depositions?size=1 |
| auth status / login / logout | Local config |
| records search (alias list) | GET /records |
| records get (alias info) / files | GET /records/:id |
| records export | GET /records/:id with requested Accept |
| records download | GET record, then its file content link |
| depositions list / get | GET /deposit/depositions[/:id] |
| depositions create | POST /deposit/depositions with {} or {metadata} |
| depositions update | PUT /deposit/depositions/:id with {metadata} |
| depositions reserve-doi | GET metadata, PUT with prereserve_doi=true |
| depositions preview | GET deposition, return browser URLs and local checks |
| depositions delete | DELETE /deposit/depositions/:id |
| depositions publish / edit / discard / newversion | POST /deposit/depositions/:id/actions/:action |
| files list / get | GET /deposit/depositions/:id/files[/:file_id] |
| files upload | GET deposition, PUT raw bytes to links.bucket/filename |
| files rename | PUT /deposit/depositions/:id/files/:file_id with {name} |
| files sort | PUT /deposit/depositions/:id/files with [{id}, ...] |
| files delete | DELETE /deposit/depositions/:id/files/:file_id |
| files download | GET deposition, then file content link |
| licenses list / get | GET /vocabularies/licenses[/:id] |
| metadata template / validate | Offline common requirements |
| api METHOD /PATH | Selected-origin REST JSON escape hatch |

Newversion POST returns the source resource per the API; the CLI follows
`links.latest_draft` and emits `{source, draft}`. If that GET fails, the error
explains that creation may already have succeeded. There is no POST retry.

The file rename documentation has inconsistent `filename`/`name` examples;
the CLI uses `{name}` as in the Python example and data example. Tests verify
that body. A real rename has not been attempted on production.

The preferred upload API is the streamed bucket PUT, including Content-Length,
rather than the older multipart endpoint with its lower documented size limit.
MD5 verification follows Zenodo's integrity checksum; downloads also understand
sha256. Legacy `files` arrays and modern `files.entries` objects are supported.
An absent checksum is reported as unverified. A mismatch does not delete the
remote upload, but returns a failure requiring inspection; download mismatches
discard the temporary file and preserve an existing destination.

## Publication and policy

Zenodo's native unpublished draft is the preparation surface. Create/update/
upload have no implicit publish path. Browser preview is `/records/:id?preview=1`
and needs an authenticated browser session; API self links are separate.
Preview's metadata checks do not promise that the server will accept publishing.

Ordinary writes use explicit commands without a global write enable flag.
Publish, discard and deletion require `--confirm ID`. This applies at the HTTP
boundary, including generic API calls. An optional `ZENODO_CLI_READ_ONLY=true`
environment lock is OR-ed with the CLI flag, so `--read-only=false` cannot
disable it. In read-only mode all non-GET/HEAD requests are rejected before IO.
There is no dry-run feature or local full Zenodo deployment.

Non-canonical paths (encoded separators, dot segments, doubled slashes) cannot
obscure actions. Only documented legacy action names/methods are admitted by
the generic action route. Requests and response links must stay within the same
scheme/host and `/api` prefix; credentials never appear in URL parameters.
GET redirects stay on-origin; mutation redirects are rejected rather than
changing methods or replaying a write. HTTPS is required except loopback HTTP
for tests. TLS verification is always enabled.

Public endpoints work anonymously; private commands require a token. Production
and sandbox environment variables apply only to their exact endpoints; custom
hosts use their own config entry populated by `auth login --token-stdin`.
Config tokens are plaintext, endpoint-scoped, mode 0600, atomically replaced.
Environment tokens take precedence. Errors redact the selected credential.

## Pagination, retries and limits

Single-page output is native. `--all` retains complete page payloads in a `pages`
array; `complete=false,next=...` marks bounded output. Deposition arrays use page
numbers and stop on a short page. Search objects use `links.next`. Repeated links
fail rather than loop. Default page size is 25; anonymous record search cannot
request above 25, authenticated requests above 100. Default bound is 100 pages.

Only GET 429/503 retries are performed (default 2, max 5). Retry-After seconds
or HTTP dates are honored up to 60 seconds; longer waits return the server
error. No network/5xx retries on mutations. Cancellation interrupts waits and
HTTP operations. Timeout applies per HTTP request, including streamed transfers.

JSON input is bounded at 8 MiB; ordinary API responses at 32 MiB; error bodies
at 64 KiB; upload metadata response at 1 MiB. File content streams without those
JSON limits. Transfers require a regular local upload file; filenames cannot
contain path separators. No bulk recursive uploads or resumable multipart flow.

## Validation boundary

HTTP tests exercise actual Cobra dispatch and transport using httptest. No test
creates or publishes production records. A configured shell token is used only
for explicitly run, GET-only smoke checks. The implementation plan and evidence
live next to this document. Full semantic schema validation, concurrent upload
locking, quota checks, DOI registration and community approval remain server
responsibilities. Name collision checks are a preflight, not a server atomic
conditional write: concurrent uploaders can race.

Live GET checks exposed two differences from the older developer reference:
file-content routes reject Accept: application/octet-stream (406), so downloads
use Accept: */*; `/api/licenses` returns 404, so dedicated license commands use
the current InvenioRDM `/api/vocabularies/licenses` resource. This is intentional
current-service compatibility rather than a claim that the old reference is
fully current. Other vocabulary types can be queried with the generic API.

Source: [official REST API](https://developers.zenodo.org/),
[native draft preview](https://help.zenodo.org/docs/deposit/create-new-upload/).
