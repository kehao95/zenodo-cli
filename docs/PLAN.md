# Implementation plan

Build an agent-facing Zenodo CLI, taking slk's Go/Cobra command hierarchy,
JSON-first pipes, human output, environment auth, and stable exit codes as the
reference. Use the documented REST deposition API, not an inferred private API.

## Delivery sequence and acceptance

1. Foundation: independently testable root command, environment-only
   credentials, typed errors, version, CI and release build.
2. HTTP boundary: HTTPS, same-origin links and redirects, explicit write commands,
   immutable environment read-only policy, confirmation for irreversible or
   destructive actions, bounded GET retries, cancellation.
3. Read workflows: records search/get/export/files/download, deposition list/get,
   license lookup, paginated aggregation retaining native page payloads.
4. Draft workflows: create, replace metadata, DOI reservation, delete, publish,
   edit, discard, new-version (return the new draft, not only source record).
5. Files: streamed bucket upload with collision protection and checksum check,
   list/get/rename/order/delete and atomic verified downloads without clobbering.
6. Escape hatch: same-policy generic REST calls, JSON file/stdin inputs,
   additional documented endpoints accessible without wrappers.
7. Validation: HTTP fixtures exercise end-to-end command dispatch, all write
   routes and confirmations, environment lock, redirects, credentials,
   pagination, error mapping, transfers and cancellation. Optional real GET-only
   smoke checks; never create test records on production.
8. Delivery: document exact commands, API coverage, limitations and validation;
   commit and push implementation to the supplied GitHub repository; register
   the repository in the personal workspace routing index.

## Boundaries

All eight delivery phases are implemented. The concrete verification results
and production-versus-fixture boundary are recorded in docs/VALIDATION.md.

- No automatic create-and-publish convenience command; upload never publishes.
- No production mutation in tests. Authentication uses only ZENODO_ACCESS_TOKEN,
  without config-file persistence or a dedicated sandbox mode.
- Metadata updates replace the supplied full object, preserving unknown fields
  exactly rather than inventing a client-side subset of Zenodo's schema.
- General JSON input receives structural validation; Zenodo owns full semantic
  validation. A local metadata validate command catches common required fields.
- Release tags are created on explicit request; package registry publishing is
  outside the initial scope.
- OAI-PMH harvesting, monthly metadata dumps and community submission moderation
  are separate workflows; GET/API escape hatch can inspect their REST surfaces.

## Completion evidence

Use httptest for automated writes and GET-only production smoke checks. The
native draft and browser preview workflow is the publication preparation path.
Do not add CLI dry-run or a local full Zenodo deployment.

See docs/VALIDATION.md for the delivered checks and observed live API behavior.
