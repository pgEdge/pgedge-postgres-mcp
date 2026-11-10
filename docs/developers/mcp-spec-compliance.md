# MCP Specification Compliance

This page records this server's compliance status against the Model
Context Protocol specification, and the decisions behind what was and
was not adopted from the latest revision, `2026-07-28`. It exists so a
future contributor can see why a given piece of the spec is or is not
implemented without re-deriving the reasoning.

## Summary

This server is **dual-era**: it implements the handshake-based
("legacy") revisions `2024-11-05`, `2025-03-26`, `2025-06-18` and
`2025-11-25` (a connection-scoped `initialize` handshake), and the
current revision, `2026-07-28` (stateless, per-request negotiation), at
the same time, on both transports. A request is served under whichever
model it uses:

* A request whose `params._meta` carries
  `io.modelcontextprotocol/protocolVersion` is served under the
  `2026-07-28` model. On HTTP, an `MCP-Protocol-Version` header also
  marks a request modern even when the body doesn't, unless the
  header names one of the legacy revisions above. A legacy Streamable
  HTTP client of `2025-06-18` or later must send that header, with its
  negotiated legacy revision, on every request after the handshake, so
  the header's value rather than its presence decides (#277). Any
  other value, including `2026-07-28` itself, marks the request
  modern; this catches a client that means to be modern but sends a
  body with no usable `_meta.protocolVersion` (omitted, or the
  reserved key misspelled), which otherwise would have been silently
  served as legacy with no indication the header was ignored. See
  `isModernHTTP` in `internal/mcp/modern.go`.
* A request without either signal, including every `initialize`
  handshake, is served under the legacy model, with the revision
  settled by the handshake as described under "Legacy revision
  negotiation" below.

This matches the versioning specification's own guidance: *"A dual-era
server MAY serve both eras concurrently on the same endpoint or
process."* Adopting `2026-07-28` in full therefore did not require
removing anything: the legacy path is unchanged, byte-for-byte, for a
request that does not opt into the modern one.

## What is implemented

* **Legacy revision negotiation.** `initialize` answers with the
  newest supported legacy revision at or below the one the client
  requests, with `2024-11-05` for a request older than every
  supported revision, and with `2025-11-25` (`ProtocolVersion` in
  `internal/mcp/server.go`) when the client omits `protocolVersion`.
  The server originally answered `2024-11-05` whatever was requested,
  which a strict Streamable HTTP client with no fallback, such as
  OpenAI Codex, refused outright because that revision predates the
  transport (#277). Only handshake-based revisions belong in
  `supportedProtocolVersions`; `2026-07-28` has no handshake, so it
  is served by the per-request path above rather than negotiated.
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
* **`404` alongside `-32601` for a modern request naming a method
  this server doesn't implement**, on HTTP. The transport spec pairs
  the two specifically so a client can tell a modern server that
  simply doesn't implement a method apart from a legacy HTTP+SSE
  server that doesn't host this endpoint at all -- both could
  otherwise return a bare `404` with nothing to distinguish them. This
  is a handler-level rejection like any other `-32601`, so it is
  checked independently of the preflight-vs-handler distinction the
  general `400` handling below relies on. `initialize` itself is one
  instance of this: it isn't a method the modern era recognizes at
  all (`2026-07-28` removed the handshake entirely), so a
  modern-shaped request naming it gets this same treatment rather
  than a legacy `InitializeResult`.

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
* **Icons** — purely cosmetic (visual identifiers for tools/resources
  in a client UI); no functional effect on protocol compliance one way
  or the other. Not adopted for this pass; nothing prevents adding it
  later without touching anything above.

## Authorisation

The server has its own OAuth 2.0 authorisation server (`internal/oauth`,
added in #288), alongside the bearer-token scheme in `internal/auth`.
It is active when authentication is enabled, the `oauth` method is not
disabled, and `http.auth.oauth.issuer` is set; see the
[OAuth guide](../guide/auth_oauth.md). The OAuth changes in
`2026-07-28` therefore apply to it, and to the CLI and web client,
which both sign in through it. Most of them bind the client rather
than the server. Their status is as follows:

* **Issuer in the authorisation response (SEP-2468).** An
  authorisation server SHOULD include the RFC 9207 `iss` parameter in
  its authorisation response, and a client MUST check a present `iss`
  against the issuer it recorded before redeeming the code. This
  server does not send `iss`, and neither the CLI nor the web client
  checks it. See "Known gaps" below.
* **Client ID Metadata Documents, and the deprecation of Dynamic
  Client Registration.** The spec deprecates RFC 7591 Dynamic Client
  Registration in favour of Client ID Metadata Documents, which both
  clients and authorisation servers SHOULD support; registration
  remains for authorisation servers that lack them. This server
  supports only Dynamic Client Registration (`/oauth/register`,
  enabled by default and controlled by `allow_dynamic_registration`),
  and does not advertise `client_id_metadata_document_supported`, so a
  conforming client falls back to registering. There is no static
  (pre-registered) client configuration yet either. See "Known gaps"
  below.
* **`application_type` (SEP-837).** A client MUST send an
  `application_type` when it registers, because an OpenID Connect
  authorisation server defaults a missing one to `"web"` and may then
  reject native-style redirect URIs. This server is not an OpenID
  Connect provider and ignores the field. The CLI and web client do
  not send it. See "Known gaps" below.
* **Credentials bound to their authorisation server (SEP-2352).** A
  client MUST key persisted credentials by the issuer, and MUST NOT
  reuse them with another authorisation server. The CLI refuses
  metadata whose issuer does not have the origin of the server it was
  pointed at, and keys its cached client ID and tokens by that
  origin, so its credentials cannot follow a changed issuer to another
  origin; see `validateMetadata` and `cacheKey` in
  `internal/chat/oauth_client.go`. The key omits the issuer's path,
  though, so the CLI meets this only in part. The web client also meets
  it only in part: it caches its registered client ID with the issuer it came
  from, and registers again when the issuer changes (see `ensureClient`
  in `web/src/lib/oauth.js`), but it keeps its tokens under one fixed
  key with no issuer. See "Known gaps" below.

## Known gaps

Unlike the section above, these are not deliberate choices -- they are
gaps found during review that this pass did not set out to fix, recorded
here so nobody has to re-derive them:

* **`Accept` header.** The transport requires a client to list both
  `application/json` and `text/event-stream`, because a conforming
  server may answer any request with either a single JSON object or a
  request-scoped SSE stream. The requirement binds the client rather
  than the server, and this server always answers with
  `application/json`, since it implements no streaming method; it
  therefore does not inspect the header, and a request that omits it
  is not rejected. The OpenAPI specification documents the header so
  the contract stays discoverable. Enforcing it would only become
  meaningful alongside a streaming response.
* **JSON-RPC batches under `2025-03-26`.** That revision requires an
  implementation to accept a JSON-RPC batch (an array of requests),
  although `2025-06-18` removed batching again. This server decodes
  each message as a single request on both transports, so a batch gets
  `-32700 Parse error`, even from a client that negotiated
  `2025-03-26`.
* **No `iss` in the OAuth authorisation response.** The authorisation
  server does not send the RFC 9207 `iss` parameter that SEP-2468 says
  it SHOULD, and the CLI and web client do not check a present one, as
  that SEP says they MUST. The CLI rejects metadata whose issuer or
  endpoints are not on its server's origin, and the web client fetches
  metadata and sends token traffic only to same-origin paths, so both
  redeem codes only with this server's own authorisation server, which
  never sends `iss`. Nothing is lost today; the check matters once a
  client can be sent to a different authorisation server.
* **No Client ID Metadata Documents.** The authorisation server
  cannot fetch a client's metadata document from a URL-formatted
  `client_id`, so it supports only the deprecated Dynamic Client
  Registration. Without static client configuration either, turning
  `allow_dynamic_registration` off leaves a client no way to obtain a
  client ID at all.
* **No `application_type` from the CLI or web client.** Both register
  without the `application_type` that SEP-837 requires. This server
  ignores the field, so nothing fails today; an OpenID Connect
  authorisation server would treat the CLI's loopback redirect URI as
  a `"web"` client's and might reject it.
* **Web client tokens not keyed by issuer.** SEP-2352 requires a
  client to keep persisted credentials per authorisation server. The
  web client keys only its client ID by issuer; its token session sits
  under a single fixed `localStorage` key. Its token traffic only goes
  to same-origin paths, but a proxy or deployment change behind that
  origin could put a different authorisation server there, and the
  stored refresh token would then be sent to it without any issuer
  comparison.
* **CLI credentials keyed by origin, not issuer.** `validateMetadata`
  compares origins only, and `cacheKey` omits the issuer's path. If
  the issuer moves from one path to another on the same origin, the
  CLI reuses its cached client ID and sends its refresh token to the
  new token endpoint, where SEP-2352 asks for fresh credentials.
* **Optional additions in the newer legacy revisions.** Negotiating
  `2025-03-26`, `2025-06-18` or `2025-11-25` does not mean the server
  implements every optional feature those revisions add; for example,
  icons are listed above as not adopted.

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
