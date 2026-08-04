# MCP Specification Compliance

This page records this server's compliance status against the Model
Context Protocol specification, and the decisions behind what was and
was not adopted from the latest revision, `2026-07-28`. It exists so a
future contributor can see why a given piece of the spec is or is not
implemented without re-deriving the reasoning.

## Summary

This server is **dual-era**: it implements its original protocol
revision, `2024-11-05` (a connection-scoped `initialize` handshake),
and the current revision, `2026-07-28` (stateless, per-request
negotiation), at the same time, on both transports. A request is
served under whichever model it uses:

* A request whose `params._meta` carries
  `io.modelcontextprotocol/protocolVersion` is served under the
  `2026-07-28` model.
* A request without it, including every `initialize` handshake, is
  served exactly as this server has always served it.

This matches the versioning specification's own guidance: *"A dual-era
server MAY serve both eras concurrently on the same endpoint or
process."* Adopting `2026-07-28` in full therefore did not require
removing anything: the legacy path is unchanged, byte-for-byte, for a
request that does not opt into the modern one.

## What is implemented

* **`server/discover`** (stdio and HTTP) — required by the spec for
  every server. Returns `supportedVersions`, `capabilities`, and
  `serverInfo`. Subject to the same per-request validation as every
  other modern method when called as one (with `_meta.protocolVersion`
  set); a client that does not yet know what this server supports can
  instead call it exactly like a legacy request (no `_meta` at all) and
  get the same answer with no validation to satisfy first, since
  `handleDiscover`/`handleDiscoverHTTP` do not vary their response by
  era.
* **Per-request `_meta` negotiation** —
  `io.modelcontextprotocol/protocolVersion`, `clientInfo`,
  `clientCapabilities`, and `logLevel` are parsed from every request's
  `params._meta`. A request whose `protocolVersion` is
  not `2026-07-28` (the only modern revision this server implements)
  gets `UnsupportedProtocolVersionError` (`-32022`); a request missing
  the required `clientCapabilities` field gets `-32602 Invalid params`.
* **`resultType`** on every modern result (always `"complete"`: this
  server never implements Multi Round-Trip Requests, so
  `"input_required"` never applies), and `_meta.serverInfo` on every
  modern result.
* **`ttlMs`/`cacheScope`** on the results the spec classifies as
  cacheable (`tools/list`, `resources/list`, `prompts/list`,
  `resources/read`). This server's registries are populated once at
  startup and never change afterward, so every list is valid
  indefinitely (`ttlMs` reflects a 24-hour horizon rather than an
  unbounded value, and `cacheScope` is always `"public"`, since nothing
  returned varies per caller).
* **Streamable HTTP request headers** — `MCP-Protocol-Version`,
  `Mcp-Method`, and (for `tools/call`, `resources/read`, `prompts/get`)
  `Mcp-Name`, including the Base64 sentinel encoding for values that
  are not safe plain ASCII. A mismatch between a header and the request
  body returns `HeaderMismatch` (`-32020`) and HTTP `400`, per spec.
  Header validation only applies to a request that is itself modern; a
  legacy request is never expected to carry these headers and is not
  checked for them.
* **`extensions`** capability field (always an empty object: this
  server implements no MCP extensions).
* **Resource-not-found as a protocol error.** Independent of the
  `2026-07-28` work, but found while implementing it:
  `resources/read` on an unknown URI previously returned a JSON-RPC
  *success* whose content happened to describe an error, on both
  registries, in every revision this server has ever shipped. This is
  now a real JSON-RPC error — `-32002` for a legacy request (the code
  every revision through `2025-11-25` defines for exactly this case)
  or `-32602` for a modern request (the code `2026-07-28` renumbered it
  to, to align with the JSON-RPC specification). See
  `mcp.ErrResourceNotFound`, `resourceNotFoundCode`, and
  `ContextAwareRegistry.Read` in `internal/resources/`.

## What was deliberately not adopted, and why

* **`subscriptions/listen`** (the replacement for the old GET/SSE
  stream and `resources/subscribe`/`unsubscribe`) — this server has no
  source of change notifications to deliver. Every tool, resource, and
  prompt is registered once at startup (built-ins are fixed in code;
  custom ones load from a definitions file read once during
  initialization) and never changes for the life of the process, so
  there is nothing a `listChanged` notification would ever report.
  Implementing a long-lived SSE endpoint with keep-alives and
  disconnect handling for a stream that can never emit anything would
  be speculative infrastructure with no current use. This server does
  not declare `listChanged`/`subscribe` in its capabilities today
  either, legacy or modern, so no client should expect it.
* **Multi Round-Trip Requests (MRTR)**, and by extension **Roots,
  Sampling, and Elicitation** — this server has never implemented any
  of the three client-side features MRTR exists to replace (it talks to
  LLM providers directly through its own proxy, not through the MCP
  sampling capability), so there is nothing to migrate. The spec also
  deprecates Roots, Sampling, and Logging outright as of `2026-07-28`;
  adopting them now to immediately migrate them to MRTR would be
  building a deprecated feature.
* **`x-mcp-header` tool parameter annotations** — explicitly optional
  for servers per spec ("use of `x-mcp-header` is optional for
  servers"). It exists to let infrastructure (load balancers, tenant
  routers) inspect a specific argument without parsing the JSON-RPC
  body; none of this server's tools have a parameter that benefits from
  that, since none of them are multi-tenant-routed.
* **OAuth-related SEPs** (issuer validation, Dynamic Client
  Registration deprecation, Client ID Metadata Documents,
  `application_type`) — this server's HTTP authentication is a simple
  bearer-token scheme (`internal/auth`), not OAuth. These SEPs govern a
  framework this server does not use.
* **Icons** — purely cosmetic (visual identifiers for tools/resources
  in a client UI); no functional effect on protocol compliance one way
  or the other. Not adopted for this pass; nothing prevents adding it
  later without touching anything above.
* **A newer legacy floor.** This server's legacy revision remains
  `2024-11-05`; it was not bumped to `2025-03-26`, `2025-06-18`, or
  `2025-11-25` on the way to `2026-07-28`. Those revisions are
  handshake-based like `2024-11-05`, so a legacy client negotiating any
  of them would see behavior this server does not implement (their own
  incremental additions -- elicitation, richer completions, and so on).
  Skipping straight to the stateless model for anything past
  `2024-11-05` avoids implementing three handshake-based revisions this
  server would then have to maintain forever, for a set of clients that
  can transparently use the stateless model instead by sending
  `_meta.io.modelcontextprotocol/protocolVersion` on their requests.

## Where the implementation lives

* `internal/mcp/modern.go` — the modern-protocol machinery: `_meta`
  parsing and era detection, `server/discover`'s data, the two new
  error types, HTTP header validation, and the result-wrapping helpers
  used by both transports.
* `internal/mcp/server.go` / `internal/mcp/http_server.go` — the
  per-request modern/legacy branch, `server/discover`'s dispatch, and
  every handler updated to send a wrapped result for a modern request.
* `internal/resources/registry.go` /
  `internal/resources/context_aware_registry.go` — the resource-lookup
  ordering fix (checking whether a URI is known *before* acquiring a
  database client, so a not-found response never depends on database
  connectivity) and the `ErrResourceNotFound` sentinel.
* `test/mcp_modern_protocol_test.go` — end-to-end coverage against the
  real server binary over real HTTP: `server/discover`'s shape, header
  validation (missing headers, mismatched `Mcp-Name`, unsupported
  version, missing `clientCapabilities`), the modern result fields, and
  confirmation that a legacy request sees none of them.
