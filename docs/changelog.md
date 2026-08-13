# Changelog

All notable changes to the pgEdge Natural Language Agent will be
documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Google Gemini now works as an embedding provider, alongside Voyage AI,
  OpenAI, and Ollama. Set `embedding.provider` to `gemini` and supply a
  key through `gemini_api_key_file`, `gemini_api_key`, or the
  `PGEDGE_GEMINI_API_KEY` and `GEMINI_API_KEY` environment variables; the
  optional `gemini_base_url` setting and the
  `PGEDGE_GEMINI_EMBEDDING_BASE_URL` variable route requests through a
  proxy. The knowledgebase gains the matching `embedding_gemini_api_key`,
  `embedding_gemini_api_key_file`, and `embedding_gemini_base_url`
  settings, together with the `PGEDGE_KB_GEMINI_API_KEY` and
  `PGEDGE_KB_GEMINI_BASE_URL` variables, and accepts `gemini` as an
  `embedding_provider`. The default model is `gemini-embedding-001`,
  which produces 3072 dimensions. Because the embedding and LLM
  configurations read the same key variables, a single Gemini key now
  drives both chat and embeddings. Knowledgebase search reads the
  `gemini_embedding` column, so it needs a `kb.db` built with Gemini;
  searching a knowledgebase that holds another provider's vectors now
  reports the mismatch instead of returning meaningless scores, and
  databases built before the column existed continue to work with the
  other providers.

- The CLI client now supports Google Gemini as an LLM provider, which
  the server has offered for some time. Select it with `-llm-provider
  gemini`, with `PGEDGE_LLM_PROVIDER=gemini`, or with `provider:
  gemini` in the configuration file, and supply the key through the
  new `-gemini-api-key` and `-gemini-api-key-file` flags, the
  `PGEDGE_GEMINI_API_KEY` or `GEMINI_API_KEY` environment variables,
  or the `gemini_api_key` and `gemini_api_key_file` settings. The
  provider can also be switched at runtime with `/set provider
  gemini`, and defaults to the `gemini-2.5-flash` model. (#238)

- Gemini can now be pointed at a proxy on the chat side, in the same way
  that Anthropic and OpenAI already could. The server gains an
  `llm.gemini_base_url` setting and the CLI client gains a
  `gemini_base_url` setting, both of which also read the new
  `PGEDGE_GEMINI_BASE_URL` environment variable, and both of which
  default to `https://generativelanguage.googleapis.com`. The embedding
  side is untouched and keeps its own separate variable, so routing chat
  traffic through a proxy leaves embedding traffic alone.

- The CLI client now accepts `-anthropic-api-key-file` and
  `-openai-api-key-file`, so that each API-key provider offers a key file
  flag rather than Gemini alone. Each behaves as the Gemini flag does: the
  file is read when the flag is applied, a direct `-<provider>-api-key`
  flag beats the corresponding file, and a path given on the command line
  that cannot be read, or that holds nothing, is an error rather than a
  silent fallback.

### Changed

- The LLM library moves to `pgedge-go-llm-lib` v0.3.0, which adds the
  `signature` field to a tool call and carries it through the streaming
  API, the SSE proxy, and the outbound Gemini request as that provider's
  `thoughtSignature`. The web client needs the field on the wire for its
  own signature handling to have anything to preserve.

- The example configuration files now show the `sslcert`, `sslkey`, and
  `sslrootcert` database settings, commented out alongside `sslmode`,
  together with the matching `PGEDGE_DB_SSLCERT`, `PGEDGE_DB_SSLKEY`,
  and `PGEDGE_DB_SSLROOTCERT` environment variables. The settings have
  always been supported; showing only `sslmode` left client certificate
  authentication undiscoverable for anyone working from the examples.

- The web client and the CLI's HTTP and stdio clients now speak the MCP
  2026-07-28 protocol exclusively, replacing the legacy `initialize`
  handshake with the stateless `server/discover` call and the
  per-request `_meta` envelope on every subsequent request. There is no
  legacy fallback, so a CLI or web client from this release requires a
  server from this release or later.

### Changed

- Dependencies across every ecosystem this project uses are now on their
  latest releases. The GitHub Actions pins move to the current major of
  each action, which also resolves the split where `actions/checkout`,
  `actions/upload-artifact`, and `actions/download-artifact` were each
  pinned at two different majors in different workflows; all of these
  majors amount to the Node 24 runtime and an ESM migration, and no input
  these workflows pass has been removed. The Go module graph is updated,
  which brings `glamour` to v1.0.0, `pgedge-go-llm-lib` to v0.2.0,
  `modernc.org/sqlite` to v1.56.0, and `fsnotify` to v1.10.1. CI now
  lints with `golangci-lint` v2.12.2, builds the web client on Node 24
  rather than the end-of-life Node 20, and builds the documentation on
  Python 3.14; the `Dockerfile.web` build and runtime stages move to
  `ubi9/nodejs-24` and `ubi9/nginx-126`. The documentation requirements
  are refreshed, except for `typing-inspect`, which
  `mkdocs-multirepo-plugin` caps below 0.9.

- The web client is now built on React 19 and MUI 9, along with Vite 8,
  Vitest 4, and the current testing-library releases. MUI has removed the
  `PaperProps` prop from `Dialog`, `Menu`, and `Popover` and `InputProps`
  from `TextField`, so those fourteen call sites now pass the equivalent
  `slotProps.paper` and `slotProps.input`, and `@mui/icons-material` no
  longer ships a `HelpOutline` alias, so the header help button uses
  `HelpOutlined`. Nothing else in the client needed migrating, since it
  lays out with `Box` and CSS grid rather than the reworked `Grid`, and
  never used the legacy `@mui/styles` APIs.

- The server and CLI client no longer send a sampling temperature, leaving
  the choice to the provider's own default. Several current models,
  including the Claude 5 family, reject the `temperature` parameter
  outright, and because the server attached one to every request the web
  client failed with `anthropic (400): temperature is deprecated for this
  model` on any such model; since the model comes from the client's
  selector rather than `llm.model`, a fresh session hit this without any
  configuration being at fault. There was no way to opt out either: the
  `llm.temperature` option defaulted to 0.7 and the configuration merge
  treated 0 as unset. That option and its `PGEDGE_LLM_TEMPERATURE`
  environment variable are therefore retired. Existing configuration files
  that still set them keep loading, the values simply having no effect.
  Callers of the LLM proxy API may still pass a per-request `temperature`,
  which is forwarded only when present.

### Fixed

- The OpenAI embedding provider no longer fails with "model parameter is
  required" when `embedding.model` is left unset. It now defaults to
  `text-embedding-3-small`, matching the built-in fallbacks Voyage,
  Gemini, and Ollama already had.

- `query_database` now caps result sets correctly even when a query's
  text merely mentions "limit" or "offset" without using them as a SQL
  clause. It previously decided a query already capped its own results
  by searching the raw statement text for those words, so a query
  filtering on a phrase like "credit limit" was wrongly treated as
  already limited and returned every matching row instead of the
  requested number. Detection now runs against the statement with
  comments and string, identifier and dollar-quoted literals removed,
  and matches the keywords as whole words, so mentions inside data,
  quoted identifiers or comments no longer suppress the safety cap.
  (#260)

- The CLI client no longer falls back to a model that cannot hold a
  conversation. When a saved model preference named something the provider
  no longer serves, and the provider's default was absent too, the client
  took the first entry in the model list without checking what kind of
  model it was; an embedding model sorting first left every message failing
  with the provider's complaint that the model does not support chat,
  whilst a usable model sat further down the same list. The fallback now
  skips the model kinds that identify themselves by name, covering
  embedding, reranking, moderation, transcription, speech, image, video and
  music models across all four providers, and takes the first entry that
  looks conversational. A model named on the command line, or held as a
  saved preference the provider still serves, is used as before. (#255)

- `select_database_connection` now connects to the target database as
  part of the switch, instead of only updating which database is
  current and leaving the connection for whatever tool call happened to
  use it next. Previously, `list_database_connections` kept reporting a
  freshly switched-to database as `unavailable` until an unrelated query
  connected it, contradicting the success response from the switch
  itself; a switch to an unreachable database also reported success and
  only surfaced the failure on first use. Switching now fails immediately
  with the connection error if the database cannot be reached. (#256)

- A configuration file that sets an LLM or embedding option without also
  naming the provider, or enabling the section, is no longer discarded.
  The merge tested only those two fields, so a file that configured just a
  base URL, a key, a key file, or a token limit was silently ignored and
  the setting never took effect. Every field in both sections now merges
  independently. The `enabled` flag is the one exception: a plain boolean
  cannot distinguish an omitted setting from a false one, so, as elsewhere
  in the merge, only a true is carried across, and a file that mentions
  neither cannot switch off a section another source enabled. (#250)

- The CLI client now lets environment variables override the
  configuration file, which is what the documentation has always
  described and what the server has always done. The client read the
  environment into its defaults before parsing the file, so the file
  quietly won instead, and the same variable resolved differently
  depending on which of the two binaries read it. Settings resolve as
  documented: command-line flags, then environment variables, then the
  configuration file, then the built-in defaults. An unset or empty
  variable still leaves a configured value alone. (#251)

- Choosing a model that cannot hold a conversation now says so. A
  provider's model list is not confined to chat models; Gemini advertises
  text-to-speech, image, music and agent models alongside the
  conversational ones, and they cannot be told apart from the metadata,
  because all of them report `generateContent` and the API describes no
  multi-turn capability. Picking one used to fail with the provider's own
  words about response modalities or an unfamiliar API, which reads like a
  misconfiguration rather than the wrong sort of model. The web client now
  recognises those replies and explains that the selected model cannot be
  used for chat, naming it and saying what it does produce where the
  provider says so. An error it does not recognise still reaches the user
  exactly as the provider phrased it. Filtering such models out of the
  list belongs with the library that serves it, and is tracked
  separately. (#246)

- Switching LLM providers and sending a message immediately afterwards no
  longer pairs the new provider with the previous provider's model. The
  model list for the newly selected provider refreshes asynchronously, but
  neither the send handlers nor the message input were gated on that
  fetch still being in flight, so a fast switch-and-send could reach the
  server as e.g. `{"provider": "openai", "model": "gemini-flash-latest"}`.
  The chat input, the Send button, and prompt execution are now disabled
  whilst the new provider's model list is loading. The same mismatch was
  reachable when that fetch failed outright, or when a provider returned
  no models at all, because sending was re-enabled whilst the previous
  provider's model was still selected; the selection is now cleared in
  both cases, which leaves the server to fall back to the provider's own
  configured default.

- Gemini tool calls now work through the web client. `sseChat.js`'s
  `tool_use_start` handler only ever populated a tool call's arguments from
  later `tool_use_delta` chunks, but Gemini delivers the complete arguments
  on the start event itself and never sends a single delta chunk (unlike
  Anthropic and OpenAI, which stream them incrementally), so every Gemini
  tool call ran with empty arguments regardless of what the model actually
  asked for; this was the root cause behind issue #235's reported retry
  loop. The handler now seeds the argument buffer from the start event when
  it already carries complete arguments. Ollama delivers its arguments the
  same way, so its tool calls are fixed by the same change. Separately, the
  handler never captured a tool call's provider signature at all, so a
  Gemini thinking model's required `thought_signature` was dropped before
  it was ever stored, breaking the next tool-calling turn. The signature is
  now carried through unchanged, and a response that has none is
  unaffected. (#244)

- The web client no longer keeps re-running a tool call that cannot
  succeed. Where a model responded to a tool error by reissuing the
  identical call, the agentic loop would repeat it up to fifty times,
  spending a request on each attempt, before failing with a bare
  message about the loop limit. The loop now stops after the same tool
  fails three times in a row with the same arguments, and explains what
  happened, naming the tool and quoting the error. A retry with
  different arguments is not affected, and a success clears the count,
  so a model that adapts is left alone.

- Switching `embedding.provider` (or `knowledgebase.embedding_provider`)
  away from Ollama without also setting a model no longer sends the
  Ollama model name to the new provider. `defaultConfig` seeded both
  `Embedding.Model` and `Knowledgebase.EmbeddingModel` with Ollama's
  `nomic-embed-text` as part of the shared baseline, and `mergeConfig`
  only overwrites a field when the loaded config sets it, so that value
  survived untouched for a config that named a provider alone. Each
  provider's own model default in `newEmbedClient` only applies when the
  model is empty, so it never got the chance to run: a live request
  with Gemini configured this way sent model `nomic-embed-text` and was
  confirmed to fail with a 404, rather than resolving to
  `gemini-embedding-001` as intended. Both fields now default to empty,
  which changes nothing for Ollama itself, since `newEmbedClient` already
  supplies `nomic-embed-text` there when the model is unset.

- The configured databases are now listed in a deterministic order, sorted
  by name. `ClientManager` holds them in a map and both accessors iterated
  it directly, so the order was randomised by the Go runtime on every call:
  the web client's database selector reshuffled between sessions, which is
  awkward to read with more than a handful of instances configured, and the
  `list_database_connections` tool returned its entries in a fresh order
  each time, invalidating the provider-side prompt cache on every request
  in the same way that `tools/list` did before (#213). The
  `/api/databases` endpoint and the database resources share the accessor,
  so all of them are stable now. The sort key is the map key, so no two
  entries can tie.

- `TestEnsureMetadataFor_SharesFailedReload` no longer fails at random. It
  released ten goroutines at once and asserted that exactly one metadata
  load attempt resulted, but the leader's attempt targets a reserved port
  and so fails immediately; whenever it finished before the rest of the
  burst had been scheduled, those callers found no reload to join, became
  leaders themselves, and the count came out at two or three. The reload is
  now driven through the leader API, as
  `TestEnsureMetadataFor_SharesInFlightReload` already does, so the
  in-flight window lasts as long as the test needs, and the callers behind
  the leader join synchronously rather than being raced into place. The
  behaviour under test is unchanged, and no production code is touched.

- The web client reports its real version again. `CLIENT_VERSION` was a
  literal in `web/src/lib/mcp-client.js` and had sat at `1.0.0-alpha5`
  since that release, through alpha6, beta1, beta2, beta3 and 1.0.0, so the
  help panel showed `Web Client: v1.0.0-alpha5` alongside `Server: v1.0.0`.
  The stale value also went out as `clientInfo.version` in every MCP
  `initialize` handshake, which is what the server's logs and traces
  recorded. It is now injected from `web/package.json`, the manifest a
  release already has to bump, so the two cannot drift apart; a test
  asserts the two agree.

### Security

- `release.yml`'s `build-web`, `build-amd64`, and `build-arm64` jobs run on
  every `workflow_dispatch`, not just tag pushes, so a manual test run on
  an arbitrary branch could write to the same npm/Go dependency caches a
  real release build reads from. Caching in `setup-node` and `setup-go` is
  now gated on `github.ref_type == 'tag'`, matching the existing
  snapshot-mode gate on the GoReleaser step itself.

- The server now validates the `Origin` header on HTTP requests and
  refuses an unexpected origin with `403 Forbidden`, closing a DNS
  rebinding exposure. A page on any site the user was visiting could
  previously drive a locally running server: the attacker points their
  own domain at `127.0.0.1`, the browser then treats the local server
  as belonging to that domain, and the page reads the responses, which
  here means the schemas and query results of every configured
  database. The check reads `Origin` alone and never `Host`, because
  under rebinding the attacker controls the hostname the browser
  connects to, so the two agree and comparing them would admit the very
  request being guarded against. Validation covers every HTTP route,
  not just the MCP endpoint, and runs ahead of authentication.

  **This can require configuration when upgrading.** With no origins
  configured the server accepts loopback origins only (`localhost`,
  `127.0.0.1`, `::1`, either scheme, any port), which covers local use
  and the bundled web client unchanged. A deployment that serves the
  web client under a real hostname must now list that origin in the new
  `http.allowed_origins` setting, or `PGEDGE_HTTP_ALLOWED_ORIGINS`, and
  will otherwise see its own browser requests refused. Listing any
  origin replaces the loopback default rather than adding to it. A
  request that carries no `Origin` header at all is unaffected, so
  command line clients, scripts and other non-browser callers need no
  change. The effective policy is reported in the server's startup
  output, and a malformed entry stops the server rather than being
  ignored. See [Security](guide/security.md) and
  [Configuration](guide/configuration.md) for details. Fixes #231.

- The write-confirmation prompt in both clients now recognises a write whose
  first keyword reads. It classified a statement by that keyword alone, so
  `SELECT ... INTO`, which creates and populates a table, a CTE carrying an
  `INSERT`, `UPDATE` or `DELETE`, and `EXPLAIN ANALYZE` of any of those, were
  all read as ordinary `SELECT`s and ran on a writable connection without the
  user being asked. This completes the fix that made custom tools confirm
  their writes: that change corrected which tools are gated, whilst this one
  corrects which statements are. Classification now runs against the
  statement's comment-stripped, literal-blanked code, so a keyword inside a
  string can no longer be mistaken for the real thing and a comment can no
  longer hide one; `SELECT ... FOR UPDATE` and `EXPLAIN` without `ANALYZE`
  are still treated as the reads they are, so the prompt keeps its meaning.
  The scanner the read-only guardrails already used has moved to
  `internal/sqltext` and is now shared with the classifier, with
  `web/src/utils/sqlText.js` mirroring it for the web client. That scanner
  now also honours backslash escapes inside an `E'...'` escape string
  constant, and only there. Reading the `\'` in `E'\''` as a doubled quote
  rather than an escape ran the literal on to the end of the statement and
  hid whatever followed it, so `SELECT E'\'' INTO backup FROM users`
  classified as a read; a backslash in a plain `'...'` literal is still an
  ordinary character, as `standard_conforming_strings` requires. The scanner
  also no longer lets a `$` that continues an identifier open a dollar-quote
  tag. PostgreSQL treats `$` as a legal, non-initial identifier character, so
  `x$tag$` lexes as one identifier, but the scanner read forward from the
  first `$` and accepted the second one as that tag's own closing mark,
  mistaking two bytes of an identifier for a tag `$tag$` and then hunting
  for its next occurrence to close the body. Planting one later in the
  statement, `SELECT 1 AS x$tag$; DELETE FROM t -- $tag$` hid the `DELETE`
  and classified as a read. The read-only guardrails benefit from the same
  correction, since a statement that hides its tail from the scanner also
  hides it from them. Note that this
  is a client-side prompt and not a security boundary: a statement whose
  writes happen inside a function it calls still reads as a `SELECT`, and
  nothing textual could tell otherwise. What prevents a write on a read-only
  connection remains the transaction access mode set by the server.

- Raised the `go.mod` floor from 1.26.1 to 1.26.5. Building this server
  with the actual go1.26.3 toolchain and running `govulncheck` in binary
  mode showed crypto/tls, net/textproto, and crypto/x509 stdlib
  vulnerabilities genuinely reachable from this server's TLS and
  HTTP-header handling; rebuilding with 1.26.5 drops that to zero. CI and
  the Dockerfiles already float on `1.26`/`golang:1.26-alpine`, so this
  sets an explicit floor rather than changing what a fresh build produces.

- Login rate limiting no longer counts against an address the caller can
  choose. The server previously read the leftmost entry of
  `X-Forwarded-For`, which is the part of that header a caller sets for
  itself, so rotating the value defeated `rate_limit_max_attempts`
  entirely, and sending someone else's address had their attempts counted
  against them. The reverse proxy configurations in these docs appended to
  the header rather than replacing it, so this needed no misconfiguration
  to reproduce. Client addresses now come from the network connection by
  default, and a new `http.client_ip` block opts into reading a forwarding
  header, which is honoured only on connections from an address listed in
  `client_ip.trusted_proxies`; `X-Forwarded-For` is read from right to
  left, past trusted entries, and every instance of the header is treated
  as one list. Candidate addresses that do not parse are discarded rather
  than becoming rate limiter keys, and IPv6 addresses are no longer
  counted twice under bracketed and unbracketed forms. Note that account
  lockout is keyed on the username and was unaffected. The nginx examples
  in the documentation now set `X-Forwarded-For` to `$remote_addr` rather
  than `$proxy_add_x_forwarded_for`.

- `count_rows`'s `where` parameter now rejects a subquery. The parameter is
  interpolated directly into the generated `SELECT COUNT(*)` statement, and
  a subquery there let a caller run a boolean- or error-based blind
  injection oracle against any table the connected role could read, not
  only the table named in the call: a crafted `where` clause used the
  returned count as a true/false signal to enumerate arbitrary data a
  character at a time. `validateReadOnlyQuery` had no way to recognise
  this, since a boolean subquery is ordinary legal SQL and not one of the
  read-only escapes it screens for. The fix, `validateCountRowsWhereClause`
  in `internal/tools/count_rows.go`, rejects a bare `SELECT` or `TABLE` in
  the clause's comment-stripped, literal-blanked residue; every documented
  use of `where` is a simple predicate that never needs either. `TABLE`
  matters because `TABLE tablename` is PostgreSQL shorthand for `SELECT *
  FROM tablename` and reads exactly the same data without the word
  `SELECT` ever appearing — an initial version of this fix that checked
  only for `SELECT` missed it. `VALUES`, the grammar's third row-returning
  form, is deliberately left unblocked: it admits no `FROM` clause, so it
  can only construct a literal row and never read a table, and unlike
  `SELECT`/`TABLE` it is not fully reserved, so it can legitimately be an
  unquoted column name. `query_database` and `execute_explain` are
  unaffected, since running arbitrary SQL, subqueries included, is their
  entire purpose. Reported and fully reproduced, including full end-to-end
  secret extraction, before this fix (issue #200); a stacked-query attempt
  against the same parameter was already correctly rejected. Calling an
  arbitrary function in `where` (for example `to_regclass('other_table')
  IS NOT NULL`, which reveals only whether a name exists in the catalogue,
  not any row's contents) and a same-table filter bypass (`where="status =
  'x' OR 1=1"`) both remain possible and are out of scope for this fix:
  neither lets a caller read another table's contents, which is what
  issue #200 reported.

- Upgraded `github.com/jackc/pgx/v5` from 5.7.6 to 5.10.0, resolving three
  advisories against the PostgreSQL driver. Two are memory-safety issues
  (GO-2026-4771/CVE-2026-33815 and GO-2026-4772/CVE-2026-33816, fixed in
  5.9.0), which `govulncheck` reports at package level, meaning it finds no
  call path to them from this code. The third is a SQL injection through
  placeholder confusion with dollar-quoted string literals
  (GO-2026-5004/CVE-2026-41889, fixed in 5.9.2), which `govulncheck` reports
  at symbol level with a reachable path from the metadata loader into the
  driver's SQL sanitiser, making it the one of the three that demonstrably
  affects this project.

- Upgraded `golang.org/x/text` from 0.35.0 to 0.40.0 (GO-2026-5970, an
  infinite loop on invalid input, reachable through connection setup) and
  `github.com/yuin/goldmark` from 1.7.8 to 1.7.17 (GO-2026-5320, cross-site
  scripting, reachable through the chat client's markdown rendering).
  `golang.org/x/net` moves from 0.51.0 to 0.57.0, clearing a DNS message
  parsing panic (GO-2026-5942) that was only ever reachable at module level.
  `golang.org/x/crypto`, `golang.org/x/sync`, `golang.org/x/sys` and
  `golang.org/x/term` move with them.

  After these upgrades `govulncheck` reports no reachable vulnerability in any
  dependency. The findings that remain are in the Go standard library and are
  resolved by building with Go 1.26.5 or later; the continuous integration
  workflows and container images track the latest 1.26 patch release, so they
  pick that up without a change here.

- The system prompt now states that everything returned by a tool is
  untrusted content: query results, table and column names, document text
  and search results are data to report to the user, never instructions to
  follow. Retrieved content is written by whoever populated the database
  rather than by the person asking the question, so a document can carry
  instructions of its own; one that asks an assistant to copy it into the
  table it came from is read again by the next session that searches for
  it. The rule applies in read-only mode and otherwise. It is a mitigation
  and not a control, since a model can be argued out of any instruction,
  which is why the documentation now states plainly that the measure which
  actually prevents such a document propagating itself is the absence of
  write access.

- Custom tools now advertise whether they can modify the database, using
  the same `readOnlyHint` and `destructiveHint` MCP annotations already
  set on `query_database`. Any `pl-do` or `pl-func` tool is treated as
  capable of writing, as is a `sql` tool whose statement is not plainly a
  read; on a connection that does not permit writes every custom tool is
  advertised as read-only. A statement that cannot be classified is
  assumed to write, so an incorrect guess costs a confirmation prompt
  rather than an unannounced write.

- The CLI and the web client now ask for confirmation before any tool call
  that the server advertises as capable of writing, rather than only before
  a write through `query_database`. A custom tool that modified data
  previously executed without a prompt in either client, because the check
  was keyed to a single tool name. Statements passed to `query_database`
  are still classified individually, so an ordinary read on a
  write-enabled connection is not interrupted, and a tool that advertises
  no annotation is not treated as a write, so the built-in read-only tools
  are unaffected. The confirmation wording is now neutral, since the
  subject may be a tool call rather than a SQL statement.

  Note that the untrusted content rule above applies only to the CLI,
  which is the only client that sends a system prompt; the web client
  sends none, so it receives neither that rule nor the pre-existing
  read-only safety instructions.

- Provider API keys are no longer disclosed in error output. When an LLM or
  embedding provider rejects a credential it commonly quotes that credential
  back in its error body: OpenAI's authentication failure names the key it
  was given, partially masked but with its opening characters intact. The
  shared pgEdge LLM library relays a provider's message verbatim into the
  error it returns, and that error reached the `/api/llm/` response body,
  the trace file, and the CLI's own output. All three are now redacted, with
  both the configured credentials and anything matching a known key format
  replaced by `[REDACTED]`; the rest of the message survives, so the
  provider, status code and reason are still reported. The `/api/llm/`
  filtering is best-effort: it inspects each response write on its own, so
  a credential split across two writes would evade it.

  The durable fix belongs in the provider client library, so that a
  credential never reaches an error value at all. What is added here is a
  filter over text that should not have contained a credential in the first
  place: it recognises the formats this project handles and the values it was
  configured with, so it is a safety net rather than a guarantee. See
  [Security](guide/security.md) for the limits.

- A malformed `Mcp-Name` header could crash the request handler with a
  panic instead of being rejected cleanly. The Streamable HTTP transport's
  Base64 sentinel encoding (`=?base64?<encoded>?=`) is unwrapped by
  checking for a matching prefix and suffix before slicing out the
  encoded middle; the prefix and suffix share a `?`, so a value shorter
  than their combined length (e.g. the literal `=?base64?=`) could match
  both checks by overlapping on that character, producing a
  negative-length slice. `recoveryMiddleware` catches the panic and
  returns a `500` rather than taking the server down, but the correct
  response to a malformed header is the spec-mandated `400
  HeaderMismatch`, not an internal error with a stack trace logged for
  every occurrence. `decodeHeaderValue` (`internal/mcp/modern.go`) now
  checks the value is at least as long as the two delimiters combined
  before matching against them.

- The Docker entrypoint's multi-database deployment mode wrote its
  generated `postgres-mcp.yaml`, plain-text database passwords included,
  with the process's default umask, leaving it group- and
  world-readable. The single-database mode, which passes the password as
  a command-line argument, was unaffected. `init-server.sh` now creates
  the file with mode `600` before writing to it, so the password is
  never briefly readable beyond the owning user. (#261)

### Fixed

- `resources/read` on an unknown URI now returns a real JSON-RPC error
  instead of a disguised success. Both resource registries
  (`internal/resources/registry.go` and the `ContextAwareRegistry`
  actually wired into the server) returned a "successful" result whose
  content was a `"Resource not found: ..."` text block, on every
  revision this server has ever shipped; per the MCP spec, an unknown
  resource URI is a protocol-level error (`-32002` for a legacy
  request, `-32602` for a modern one -- see the `Added` entry below).
  `ContextAwareRegistry.Read` also now checks whether a URI is even a
  recognised resource *before* acquiring a database client, rather
  than after: previously, reading an unknown URI without a working
  database connection returned a database-error message instead of a
  not-found one, since acquiring a client came first regardless of
  whether the resource existed at all.
- The conversation actions menu in the status banner header no longer
  offers a "Delete conversation" item that does not delete anything. That
  action resets the chat window and starts a new conversation; it never
  removed the conversation from the conversation list, so a selected
  conversation appeared to survive the delete. The item is now labelled
  "Clear conversation", and its confirmation dialog explains that the
  conversation stays in the list, where the per-conversation delete button
  removes it for good. (#223)

- Tool calls made over the streaming chat endpoint are no longer silently
  dropped, which showed up in the web client as "No response received"
  whenever a provider decided to call a tool. The terminating `done` frame
  of that stream carries a chunk structure with no field for a stop
  reason, so the client had nothing to read and fell back to assuming
  `end_turn`; the agentic loop then looked only for text blocks and
  ignored the `tool_use` block sitting alongside them. The OpenAI
  provider hit this on every tool call, because it reports a distinct
  `tool_calls` finish reason that had no way of reaching the client. The
  client now infers `tool_use` when the assembled response contains a
  `tool_use` block and the server reported `end_turn` or nothing at all,
  leaving a more specific reason such as `max_tokens` untouched.

- Reading an MCP resource after an idle period no longer reports the
  database as unavailable. Metadata expires once it is older than
  `metadata_ttl` (five minutes by default), and `IsMetadataLoaded()`
  returns false both for a connection that has never loaded any metadata
  and for one whose metadata has simply aged out. The resource path
  treated the second case as a database that was not ready, so it
  returned a retryable `DATABASE_NOT_READY` without attempting a reload;
  the web client's status banner then showed "Database is switching",
  retried five times, and settled on "Database switch taking longer than
  expected", all against a perfectly healthy database. The MCP tools
  already reloaded in this situation, so only resources were affected.
  Both resource paths now reload expired metadata through a new
  `EnsureMetadata` helper, and report `DATABASE_NOT_READY` only when that
  reload genuinely fails. Raising `metadata_ttl` is no longer needed as a
  workaround. At most one reload per connection runs at a time, and
  callers arriving whilst one is in flight wait for it and share its
  outcome, so a burst arriving once the TTL has expired issues one catalog
  query between them rather than one each; the status banner refreshing
  several resources at once makes such bursts routine. Sharing the outcome
  matters most when the database is unreachable, since a retry per caller
  would queue one connect timeout each. Failures are not cached, so the
  next caller to arrive starts a fresh attempt.

- `make test` now runs every package that has tests. Ten packages were absent
  from the server target and so were never exercised by the suite or by
  continuous integration: `api`, `compactor`, `conversations`, `definitions`,
  `httperror`, `llmtracing`, `logging`, `prompts`, `search` and `tsv`. All of
  them pass; they were simply never run.

- `tools/list`, `prompts/list`, and `resources/list` now return their
  entries in a stable, sorted order on every call. Each registry stored
  its entries in a Go map and built the response by iterating it
  directly, so the order was randomised by the runtime and changed from
  one call to the next even when the underlying set was unchanged. Tool,
  prompt, and resource definitions sit at the front of the prompt sent to
  the model, so the reshuffling invalidated the provider-side prompt
  cache on every request rather than only when the set actually changed,
  and it defeated client-side diffing of the advertised list for the
  same reason. `Registry.List()` in each of `internal/tools`,
  `internal/prompts`, and `internal/resources` now sorts by name (tools,
  prompts) or URI (resources) before returning; `ContextAwareRegistry.List()`
  in `internal/resources`, which merges a fixed built-in entry with a map
  of custom resources, sorts its combined result the same way. Every
  registration is keyed by its own name or URI in this codebase today,
  so the sort has no ties to break in practice, but the comparison also
  falls back to the registration key, which is unique by construction:
  `sort.Slice` gives no ordering guarantee between two entries that
  compare equal, so a future entry whose Definition happened to
  advertise the same name or URI as another under a different key would
  otherwise be exactly as nondeterministic as before this fix. Fixes
  #211.

- `initialize` no longer echoes back whatever `protocolVersion` a client
  requests. Version negotiation is the server's half of the MCP
  handshake: the client proposes a revision, and the server is supposed
  to reply with the revision it will actually speak, which the client
  then checks against what it supports. Echoing the request answers
  with no information at all, so a client asking for a revision this
  server does not implement was told that revision was accepted, and
  found out otherwise later, against a missing capability, rather than
  at the version check meant to catch exactly this case. `initialize`
  (stdio) and `initialize` over HTTP now both call a shared
  `NegotiateProtocolVersion`, which returns the newest revision this
  server supports at or below the client's request, or the oldest
  supported revision if the request predates everything the server
  implements. This server currently implements one revision,
  `2024-11-05`, so today every negotiation converges on that value
  regardless of what a client asked for; the function is structured to
  extend cleanly if a second revision is added later. The bundled CLI
  and web client are unaffected, since both already request
  `2024-11-05` exactly.

  `initialize` over HTTP also now rejects malformed parameters with a
  proper `-32602 Invalid params` error, matching every other HTTP handler
  and the stdio transport's existing behaviour. The HTTP handler
  previously never read the client's parameters at all, so it accepted
  anything; it now parses them, and a field of the wrong JSON type, such
  as a numeric `protocolVersion` or a `clientInfo` that is not an object,
  fails the handshake instead of silently falling back to the server's
  default. Omitting the parameters entirely still succeeds and yields
  that default. Fixes #212.

### Added

- The server now implements the current MCP specification revision,
  `2026-07-28`, alongside its original revision, `2024-11-05`, on both
  transports at once (dual-era, per the versioning spec's own
  guidance). A request whose `params._meta` carries
  `io.modelcontextprotocol/protocolVersion` is served under the new,
  stateless model: `server/discover` (required by the spec for every
  server), per-request `_meta` negotiation in place of the removed
  `initialize` handshake, `resultType`/`ttlMs`/`cacheScope` on results,
  and the Streamable HTTP transport's required
  `MCP-Protocol-Version`/`Mcp-Method`/`Mcp-Name` headers. A request
  without that `_meta` field, including every `initialize` handshake
  -- which covers the bundled CLI and web client, and every existing
  integration test -- is served exactly as before this change, with no
  observable difference. Conversely, an `initialize` request that does
  carry that `_meta` field is rejected with `-32601 Method not found`
  on both transports, matching the modern era's method set, which has
  no handshake to answer -- and, on HTTP, this and every other
  `-32601` for a modern request now pairs with HTTP `404`, per the
  transport spec's own reasoning: it is what lets a client tell this
  server apart from a legacy HTTP+SSE server that doesn't host the
  endpoint at all. A present `MCP-Protocol-Version` header now also
  marks an HTTP request modern even when its body doesn't, since no
  client older than `2025-06-18` ever sends that header; a modern
  `ping` over stdio now carries `resultType` like every other modern
  result, matching what it already carried over HTTP. See [MCP
  Specification
  Compliance](developers/mcp-spec-compliance.md) for the full
  negotiation rules and, importantly, what was deliberately not
  adopted from this revision and why (`subscriptions/listen`, Multi
  Round-Trip Requests/Roots/Sampling/Elicitation, `x-mcp-header`,
  OAuth-related changes, and icons): none of them have anything for
  this server to attach to today.

- A `make vulncheck` target runs `govulncheck` over the module, using
  call-graph analysis to prioritize known vulnerabilities that this code can
  actually reach over every advisory affecting a dependency. See
  [Development](contributing/development.md) for details.

- Database configuration now accepts `sslcert`, `sslkey`, and
  `sslrootcert` fields, letting the server authenticate to PostgreSQL
  with a client certificate instead of, or alongside, a password.
  Configure them with `databases[].sslcert`, `databases[].sslkey`,
  and `databases[].sslrootcert` in the configuration file, the
  `-db-sslcert`, `-db-sslkey`, and `-db-sslrootcert` CLI flags, or the
  `PGEDGE_DB_SSLCERT`/`PGSSLCERT`, `PGEDGE_DB_SSLKEY`/`PGSSLKEY`, and
  `PGEDGE_DB_SSLROOTCERT`/`PGSSLROOTCERT` environment variables.
  `sslcert` and `sslkey` must be set together. `sslrootcert` takes
  effect only under `sslmode` `require`, `verify-ca`, or `verify-full`;
  under `disable`, `allow`, or `prefer` (the default) pgx never checks
  the server certificate against it and silently ignores the value,
  matching libpq and `psql`. In HTTP mode, changing any of these fields and
  reloading the configuration (`SIGHUP`) now closes pooled per-token
  connections so they reconnect with the new certificate settings.

- A configurable per-attempt timeout bounds each individual HTTP attempt
  to an LLM or embedding provider, so a single slow attempt becomes
  retryable instead of consuming the whole request budget; the
  knowledgebase embedding path honours the same setting. Configure it
  with `per_attempt_timeout` in the `llm` and `embedding` config
  sections, `embedding_per_attempt_timeout` in the `knowledgebase`
  section, or the `PGEDGE_LLM_PER_ATTEMPT_TIMEOUT`,
  `PGEDGE_EMBEDDING_PER_ATTEMPT_TIMEOUT`, and
  `PGEDGE_KB_EMBEDDING_PER_ATTEMPT_TIMEOUT` environment variables
  (default 60 seconds; set the corresponding environment variable to 0
  to disable the cap).

- Similarity search now validates the query embedding dimension against
  the target vector column before querying, returning a clear error on a
  mismatch instead of a raw database error.

- Similarity search now supports pgvector `halfvec` columns; it detects
  the column type and casts the query vector accordingly (requires
  pgvector 0.7.0 or later).

- The web client now uses the provider display name reported by the
  proxy, falling back to its built-in labels when none is supplied.

- Each built-in tool, resource, and prompt can now be enabled or
  disabled via an environment variable in addition to the
  `builtins` section of the configuration file. The variables are
  `PGEDGE_BUILTIN_TOOL_*`, `PGEDGE_BUILTIN_RESOURCE_*`, and
  `PGEDGE_BUILTIN_PROMPT_*`; see the configuration reference for
  the complete list. This is useful in containerized deployments
  where editing the configuration file is awkward. (#139)

### Changed

- The LLM provider clients (Anthropic, OpenAI, and Ollama) now use the
  shared
  [`pgedge-go-llm-lib`](https://github.com/pgEdge/pgedge-go-llm-lib)
  library instead of hand-rolled HTTP wire code; approximately 1500
  lines of provider-specific code are removed from `internal/chat/`.
  Behaviour is preserved; the `LLMClient` interface is unchanged.

- Anthropic prompt caching now covers both the tools block and the
  system prompt (the library exposes a `WithSystemCaching` builder
  alongside `WithToolCaching`). Long system prompts no longer pay
  full input-token cost on every turn.

- OpenAI models that require the Responses API (`gpt-5-*`, `o1-*`,
  `o3-*`) are now supported transparently; the library routes them
  to `/v1/responses` automatically based on the model name.

- Embedding provider clients (Voyage, OpenAI, Ollama) now use the
  shared
  [`pgedge-go-llm-lib`](https://github.com/pgEdge/pgedge-go-llm-lib)
  library instead of hand-rolled HTTP wire code. Approximately 1100
  lines of provider-specific code are removed from
  `internal/embedding/`. The `Provider` interface and `NewProvider`
  factory are preserved; tool consumers (search_knowledgebase,
  generate_embedding, similarity_search) compile unchanged.

- `Provider.Dimensions()` is now lazily populated from the first
  successful `Embed` call; it returns 0 before any embedding has been
  generated (previously the value was hard-coded per known model).

- Refactored `Client.LoadMetadataFor` in
  `internal/database/connection.go`. The CTE-based metadata query
  now lives in `internal/database/load_metadata.sql` and is loaded
  via `//go:embed`; the per-row scan and the grouping/transform
  logic are split into `scanMetadataRow` and `buildTableInfo` in
  `internal/database/metadata.go`. `buildTableInfo` is pure and is
  covered by table-driven unit tests that do not require a live
  database. No behavior change. (#153)

- The built-in `pg://system_info` resource now uses the machine-safe
  name `postgresql_system_info` (previously
  `"PostgreSQL System Information"`). The new name matches the
  identifier pattern enforced by Anthropic's tool-name validation
  (`^[a-zA-Z0-9_-]{1,128}$`), so the resource no longer breaks
  interoperability when a downstream MCP client forwards built-in
  capability names as provider tool names. The resource URI is
  unchanged. (#139)

- The KB Builder (formerly `cmd/kb-builder` and the
  `internal/kb*` packages) has moved to a standalone project at
  [`pgedge-ai-kb`](https://github.com/pgEdge/pgedge-ai-kb). The
  binary is renamed from `pgedge-nla-kb-builder` to
  `pgedge-ai-kb-builder`. The MCP server itself is unaffected; it
  continues to consume a pre-built `kb.db` at runtime. The Docker
  build now downloads `kb.db` from
  `https://github.com/pgEdge/pgedge-ai-kb/releases/download/kb-latest/kb.db`
  by default; pass `KB_SOURCE` to override. The
  `pgedge-nla-kb-builder_*` release archives are no longer published
  from this repository.

- The LLM HTTP proxy is now provided by `pgedge-go-llm-lib`'s
  `llm/proxy` package, mounted at `/api/llm/`. The endpoints moved
  from `/api/llm/{providers,models,chat}` to `/api/llm/v1/*`, and
  the request/response wire format now uses typed content blocks
  (see the library's `llm.ChatRequest` and `llm.ContentBlock`).
  `internal/llmproxy/` is deleted; tracing plumbs through proxy
  hooks via the new `internal/llmtracing` package.

- A streaming chat endpoint `/api/llm/v1/chat/stream` (SSE) is now
  exposed alongside the non-streaming endpoint. The non-streaming
  `/v1/chat` endpoint remains available for callers that prefer it.

- The web chat interface now consumes the streaming endpoint
  `/api/llm/v1/chat/stream` (Server-Sent Events) and renders the
  assistant response incrementally as chunks arrive. The
  non-streaming endpoint stays available for callers that prefer
  it. A new `web/src/utils/sseChat.js` helper handles the SSE
  parsing and assembles the final response into the same shape
  the non-streaming endpoint returns, so the agentic chat loop is
  unchanged.

- The tools `search_knowledgebase`, `generate_embedding`, and
  `similarity_search` now construct their embedding client
  directly via `llm.NewClient` rather than going through the old
  `embedding.NewProvider` wrapper. The `internal/embedding/`
  package is deleted entirely.

- The temporary `chat.LLMClient` interface and `libClient` adapter
  added in the first migration PR are removed. The CLI chat client
  now consumes `pgedge-go-llm-lib`'s `llm.Client` API directly.
  `internal/chat/llm.go` and `internal/chat/llm_translate.go` are
  deleted; messages and content blocks flow through the chat
  package as `llm.Message` and `llm.ContentBlock` rather than
  chat-package wrapper types. The library's `llm.Client` API
  itself is unchanged. The CLI's debug-mode HTTP tracing still
  injects via `llm.Options.HTTPClient`.

- Saved conversations from earlier versions are migrated on load:
  messages with a plain-string `content` field are wrapped as a
  single text content block, and tool-result messages saved with
  role `"user"` are promoted to role `"tool"` to match the
  library's expected shape. The on-disk JSON written by this and
  later versions uses the typed content-block format directly.

- The web client now groups the conversation-level Save and Delete
  actions in a new menu in the status banner header, alongside the
  database switcher and connection details, rather than placing them
  next to the message input. This keeps the destructive Delete action
  away from the Send button and clarifies that the actions affect the
  whole conversation. Deleting a conversation now requires confirmation
  via a dialog instead of a browser prompt. (#73)

- Custom `pl-func` tools now fail immediately, with an explanation, on
  a database connection that does not permit writes. Such a tool
  creates and drops a temporary function, which a read-only
  transaction cannot do, so it previously failed partway through with
  an opaque permissions error that invited disabling read-only mode as
  the remedy. Use a `pl-do` tool on a read-only connection.

- The read-only statement guard no longer rejects a query merely for
  mentioning `transaction_read_only` or `default_transaction_read_only`
  inside a string literal or a comment, so an ordinary lookup such as
  `SELECT * FROM config WHERE key = 'transaction_read_only'` is now
  permitted. A rejection additionally requires a construct capable of
  changing a setting.

### Security

- Read-only connections no longer accept more than one SQL statement
  per request. `query_database` previously executed any statement that
  did not begin with `SELECT`, `WITH`, `TABLE`, or `VALUES` through
  pgx's `Exec`, which falls back to the PostgreSQL simple query
  protocol whenever no bind parameters are supplied. That protocol
  accepts several semicolon-separated statements in one message, so a
  caller could append their own statements to a request, including
  `SET TRANSACTION READ WRITE` or `COMMIT; BEGIN READ WRITE`, and then
  write to the database. On a read-only connection every statement now
  runs through the extended query protocol, which carries exactly one
  statement per message and rejects anything else. Note that a leading
  comment was enough to reach the vulnerable path, so no unusual
  keyword was required. Connections configured with
  `allow_writes: true` keep the previous behaviour, including support
  for multi-statement scripts.

- The read-only statement guard now recognises the transaction access
  mode, which it previously ignored altogether: it matched only the
  literal strings `transaction_read_only` and
  `default_transaction_read_only`, and never `READ WRITE`. It now also
  rejects `SET SESSION CHARACTERISTICS`, `RESET ALL`, `DISCARD`,
  transaction control statements, `SET ROLE`,
  `SET SESSION AUTHORIZATION`, `ALTER ROLE`, `ALTER USER`,
  `ALTER DATABASE`, and
  operations whose effects fall outside the transaction and which a
  read-only transaction therefore does not prevent: `DO` blocks,
  `COPY ... TO PROGRAM`, the server-side file functions, and `dblink`.
  Statements are matched after comments and literals have been
  stripped, so a comment can no longer be used to split a keyword, and
  the guard now runs on the `count_rows` `where` argument and the
  `execute_explain` query as well as on `query_database`. Rejected
  statements are logged in full, since a rejection records an attempt
  to escape read-only mode.

- The session-level `default_transaction_read_only` setting is now
  re-applied when a pooled connection is released, and a connection
  whose state cannot be confirmed is discarded rather than reused.
  Previously the setting was applied only when the connection was
  established, so a `RESET ALL` or `DISCARD ALL` persisted on that
  pooled connection for every later caller that received it. This was
  not a route through `query_database`, which set the access mode on
  each of its own transactions and so remained protected; it mattered
  for any path that did not, such as the custom tool executor's
  `pl-func` type, and it left the session-level layer disabled for the
  rest of that connection's life.

- The custom tools framework now has end-to-end test coverage. It is
  invisible until an operator sets `custom_definitions_path`, so none of its
  three tool types had ever been exercised through the MCP protocol, which is
  how a tool type came to depend on a single guardrail without anyone
  noticing. `TestCustomToolsRespectReadOnly` enables the framework the way an
  operator would and checks each type against a read-only connection: `sql`
  and `pl-do` tools read successfully and are refused when they write, and a
  `pl-func` tool is refused up front. Each case also verifies out of band that
  the database was not modified and that no temporary function survived.

- Custom `pl-do` tools no longer interpolate arguments between fixed
  dollar-quote delimiters. The wrapper used `$mcp_custom_tool$` and
  `$mcp_args$`, and JSON encoding does not escape a dollar sign, so an
  argument value containing either delimiter closed the quoting early
  and had the remainder of the value parsed as SQL. Combined with the
  simple query protocol used for these statements, that was a complete
  bypass of read-only mode that the statement guard never saw.
  Delimiters are now generated per invocation, for `pl-func` tools as
  well, and an argument that carries one is refused.

- Read-only transactions now request their access mode as part of
  `BEGIN` rather than issuing `SET TRANSACTION READ ONLY` as a
  following statement, in `query_database`, `count_rows`,
  `execute_explain`, and the custom tool executor. The transaction is
  therefore never briefly writable, and the mode cannot fail to apply
  independently of the transaction starting.

### Fixed

- Metadata loader no longer emits duplicate column entries for a
  column that participates in more than one foreign-key constraint.
  The `fk_columns` CTE produced one row per foreign key, so the
  downstream LEFT JOIN multiplied the per-column rows; `get_schema_info`
  consequently listed the affected column once per foreign key. The CTE
  now aggregates every reference into one ordered, de-duplicated array
  per column, and `ColumnInfo.ForeignKeyRefs` is a `[]string` so all
  references are surfaced (comma-separated in the `fk_ref` output
  column) rather than silently discarding all but one. (#171)

- The edit and delete icons in the conversation history list no longer
  overlap the conversation title; the list item now reserves enough
  space for both controls so long titles ellipsize cleanly. (#73)

- Every HTTP error response is now a consistent JSON object
  (`{"error": "..."}`) with an appropriate status code, including
  framework-level cases that previously bypassed the normal handlers
  and returned a plaintext or empty body: an unknown route (404), a
  method mismatch (405), an oversized request body (413, distinguished
  from other body-read failures), and a panic inside a handler (500;
  previously the connection was simply closed with no response at
  all). A shared `internal/httperror` helper backs the new panic
  recovery and 404 catch-all middleware, as well as the handlers that
  previously wrote plaintext errors via `http.Error`
  (`/mcp/v1`, `/api/chat/compact`, `/api/openapi.json`, and the
  session-auth wrapper). Request bodies on `/api/chat/compact`,
  `/api/databases/select`, and the `/api/conversations*` endpoints are
  now also capped at 10MB, matching the existing `/mcp/v1` limit. The
  HTTP server now sets `ReadHeaderTimeout`, `ReadTimeout`, and
  `IdleTimeout` to guard against slow-header and slow-body attacks;
  these fire before a request reaches a handler, so (unlike the cases
  above) there is no response body to produce. Every 405 response now
  also sets the `Allow` header naming the supported method(s), per
  RFC 7231 §6.5.5. `GET /api/databases`'s 405 uses the shared
  `internal/httperror` writer; `POST /api/databases/select`'s 405 uses
  the endpoint's own documented `{"success": false, "error": "..."}`
  shape instead, matching its other error responses (400, 404, 403)
  rather than the bare `{"error": "..."}` it previously returned only
  for that one status code. (#189)

- Tool and resource responses now show the operator-configured database
  display name instead of the raw connection details. Previously,
  `query_database`, `get_schema_info`, `execute_explain`, `count_rows`,
  and `similarity_search` only masked the password in the connection
  string they showed the caller, leaving the real host, port, and
  database name visible; `pg://system_info` was worse, reporting the
  live-resolved server address from `inet_server_addr()`, which can be
  an internal-only address (a container or pod IP) that differs from,
  and may be unreachable via, the address the operator actually
  configured. A new `Client.DisplayName()` now backs every one of
  these responses with the connection's configured `name` (falling
  back to a password-masked connection string when none is
  configured); `pg://system_info` gains a `connection_name` field and
  its `host`/`port` fields now reflect the configured values rather
  than a live-resolved one. Ad-hoc connection strings a caller types
  inline (the `postgres://...` mini-DSL supported by `query_database`)
  are intentionally left as-is, since echoing back what the caller
  themselves supplied is not a leak. (#187)

- The token and user file watchers now detect changes delivered through an
  atomically-swapped symlink, such as a Kubernetes-projected Secret or
  ConfigMap volume, or any tool that renames a new version into place.
  Previously the watcher matched events by exact filename and only handled
  `Write`/`Create`, so a symlink swap on a different directory entry (for
  example Kubernetes' own `..data` symlink) never triggered a reload and
  updates only took effect on restart. The watcher now reacts to any event
  in the watched directory and re-resolves and hashes the watched path's
  content to decide whether a reload is warranted, catching changes that
  never touch the watched filename directly while still ignoring
  unrelated activity elsewhere in the directory. (#186)

- Metadata loader now tolerates tables with zero columns
  (e.g. `CREATE TABLE foo()`). The query LEFT JOINs against the
  per-column catalog, so a zero-column table produced a row whose
  `column_name`, `data_type`, and `is_nullable` were all NULL; the
  scan declared those targets as plain `string` and aborted with
  `cannot scan NULL into *string`, failing the entire metadata load
  and surfacing as the misleading `no database connection
  configured for this token` error. The three columns are now
  scanned as `sql.NullString` and zero-column tables appear in the
  metadata with an empty `Columns` slice. (#126)
- HTTP transport now returns `202 Accepted` with an empty body for
  JSON-RPC notifications, per JSON-RPC 2.0 §4.1 and the MCP streamable
  HTTP transport spec. Previously, the server replied to notifications
  with a `200 OK` response that had no `id` field, which is itself not
  a valid JSON-RPC message and caused strict clients (such as the .NET
  MCP SDK) to throw on every notification. Unknown notification methods
  are now also acknowledged silently rather than receiving a `-32601`
  error reply. (#142)

- Stdio transport now correctly distinguishes JSON-RPC notifications
  (no `id` member) from requests with an explicit `"id": null` (per
  JSON-RPC 2.0 §4.1). A request with `"id": null` targeting an unknown
  method previously matched the same `req.ID == nil` guard used to
  suppress notification replies and was silently dropped; it now
  receives the required `-32601 Method not found` response. The
  hardcoded `notifications/initialized` case was likewise affected and
  is now filtered uniformly with all other notifications at the read
  loop, using the same `hasIDField` raw-bytes probe introduced for the
  HTTP transport in #142. (#152)

- JSON-RPC response now always serializes the `id` field, including
  when it is null. Per JSON-RPC 2.0 §5.1, the response object MUST
  include the id member; the value is the originating request's id, or
  null when the id cannot be determined (parse error / invalid
  request) or when the request itself used `"id": null`. The
  `JSONRPCResponse.ID` JSON tag previously used `omitempty`, which
  caused Go's encoder to drop the field for nil interface values —
  producing a response without an `id` field, which is itself a
  malformed JSON-RPC body. This affects both the HTTP and stdio
  transports. (#152)

- Database switching via `select_database_connection` now persists
  correctly in HTTP mode for unbound API tokens.
  `GetAccessibleDatabases` previously returned only the first
  configured database for unbound tokens, causing `getClient` to
  silently override the user's selection on every subsequent tool
  call. The method now returns all databases, matching the behavior
  of `CanAccessDatabase`. (#117)

- Added a JSON-RPC `ping` handler on both stdio and HTTP transports
  so MCP clients that issue `ping` during initialization or health
  checks receive a compliant `{}` result instead of a
  `-32601 Method not found` error. The stdio handler suppresses
  responses to notification-style pings (no `id`) per JSON-RPC
  2.0 §4.1. (#167)

### Added

- The installer detects running Postgres instances and offers
  to connect to them, with automatic database listing.

- Added `--detect` / `-Detect` flag for non-interactive
  auto-connection to detected Postgres instances.

- The installer detects previous installations and offers
  to update the binary or reconfigure the database connection
  instead of re-running the full install flow.

- Schema metadata cache now refreshes automatically based on a
  configurable TTL. The `metadata_ttl` database option controls
  how long cached metadata remains valid (default: 5 minutes).
  This fixes an issue where `get_schema_info` returned stale
  results when tables were created outside the MCP server or
  when using read-only database connections.

- HTTP authentication is now configurable in Docker deployments
  via the `PGEDGE_AUTH_ENABLED` environment variable. Auth remains
  enabled by default; set `PGEDGE_AUTH_ENABLED=false` only in
  trusted local development environments (for example, when
  connecting Claude through `mcp-remote` with a fixed bearer
  token and needing access to multiple databases). The setting is
  honored by both the single-database and multi-database
  initialization paths. (#167)

- Google Gemini is now a supported LLM provider. Configure via
  `gemini_api_key` / `gemini_api_key_file` in the config file or
  via the `PGEDGE_GEMINI_API_KEY` / `GEMINI_API_KEY` environment
  variables.

### Fixed

- Fixed port detection on Windows; the installer now reliably
  detects Postgres instances on all network addresses.

## [1.0.0] - 2026-03-27

### Changed

- Docker container now defaults to stdio mode instead of HTTP
  mode. HTTP mode requires setting `PGEDGE_HTTP_ENABLED=true`.
  This allows the Docker image to work with stdio-based MCP
  clients such as the Docker Desktop MCP Toolkit, Claude Code,
  and Claude Desktop.

- Docker init script output now goes to `stderr` instead of
  `stdout`; this keeps `stdout` clean for the MCP protocol in
  stdio mode.

- User and token initialization (`INIT_USERS`, `INIT_TOKENS`)
  now only runs when HTTP mode is enabled. Stdio mode does not
  use HTTP authentication.

- Quickstart demo files (`docker-compose.yml`, `.env.example`,
  `pgedge-ait-demo.sh`) are now served from the GitHub
  repository instead of `downloads.pgedge.com`. The Northwind
  example database download is unchanged.

### Fixed

- Queries with trailing semicolons no longer produce a SQL syntax
  error when the server auto-appends a `LIMIT` clause. The server
  now strips trailing semicolons before appending `LIMIT`/`OFFSET`.

- MCP tools (`query_database`, `count_rows`, `get_schema_info`) now
  load metadata synchronously on the first call instead of returning
  a "database is still initializing" error. This eliminates the
  unnecessary LLM retry that previously occurred on every first
  tool call.

- Database connection timeout now defaults to 10 seconds instead of
  blocking for 60+ seconds when a target host is unreachable. A new
  `connect_timeout` configuration option allows customization of
  the timeout duration.

### Security

- The server now rejects queries that reference the
  `transaction_read_only` or `default_transaction_read_only`
  settings when the database connection is in read-only mode.
  This prevents single-statement bypass attacks (such as
  PL/pgSQL `DO` blocks with `set_config()`) that could
  circumvent the `SET TRANSACTION READ ONLY` guardrail.

- The system prompt sent to all LLM providers (Anthropic,
  OpenAI, Ollama) now includes explicit safety instructions
  that forbid attempts to bypass read-only mode when the
  database connection does not allow writes.

### Added

- MCP tool selection guidance for AI agents. The server now sends
  a server-level `instructions` field during the MCP initialize
  handshake, directing agents to prefer MCP tools over `psql` and
  shell commands. Tool descriptions include explicit "use this
  instead of..." language to steer tool selection. A new
  documentation page covers recommended `CLAUDE.md` and
  `.cursorrules` configuration for reinforcing tool preference.

- Cursor IDE plugin manifest and setup guide.

- OpenAPI 3.0.3 specification and interactive API browser. The
  server now provides a programmatic OpenAPI specification covering
  all REST endpoints. The specification is available at the
  `/api/openapi.json` endpoint (no authentication required),
  through the `-openapi` CLI flag, and as a static file in the
  documentation. Use `make openapi` to regenerate the static
  copy. The MkDocs site includes a ReDoc-powered interactive API
  browser under For Developers. API responses include an RFC 8631
  `Link` header for automatic discovery by tools such as
  `restish`.

- Write query confirmation in the CLI and Web UI. When a database
  has write access enabled, the user is prompted to confirm DDL and
  DML queries before the server executes the queries. Declining a
  query returns an error to the LLM without executing the query.

- MCP tool annotations on the `query_database` tool. The server
  sets `destructiveHint` and `readOnlyHint` annotations per the
  MCP 2025-03-26 specification; third-party MCP clients that
  support annotations can use the annotations to prompt for user
  confirmation.

- Partitioned table support in `get_schema_info`. The tool now
  recognizes partitioned parent tables (shown as `PARTITIONED TABLE`)
  and hides child partitions by default. Use the new
  `include_partitions` parameter to reveal child partitions when
  needed. This reduces context window usage for databases with
  daily or other time-based partitioning schemes.

- Example DBA toolkit as a drop-in YAML custom definitions file
  (`examples/pgedge-postgres-mcp-dba.yaml`). The toolkit provides
  three pl-do tools: `get_top_queries` for top resource-consuming
  query analysis, `analyze_db_health` for seven-category database
  health checks, and `recommend_indexes` for two-tier index
  recommendations with optional HypoPG simulation.

- Multi-host database connection support for high availability
  and failover. The `hosts` array replaces the single `host` and
  `port` fields when connecting to multiple PostgreSQL servers.
  The server generates a libpq-compatible multi-host connection
  string and passes the list to pgx for automatic failover.

- The `target_session_attrs` option controls read-write routing
  for multi-host connections. Accepted values include
  `any`, `read-write`, `read-only`, `primary`, `standby`, and
  `prefer-standby`.

- Pool health check and connection lifetime settings for
  database connections. The `pool_health_check_period` option
  sets the interval for background health checks; the
  `pool_max_conn_lifetime` option sets the maximum age of a
  pooled connection before the server closes the connection.

- The `PGEDGE_DB_HOSTS` environment variable configures
  multi-host connections as a comma-separated list of
  `host:port` pairs.

- The `--db-hosts` CLI flag specifies multiple database hosts
  as a comma-separated `host:port` list. The
  `--db-target-session-attrs` flag sets the session routing
  attribute for multi-host connections.

- Configuration validation rejects entries that specify both
  the single-host `host` field and the multi-host `hosts`
  array. The validator also checks that `target_session_attrs`
  contains a recognized value.

- The web client displays multi-host connection details in the
  database selector, showing each configured host and port
  alongside the connection status.

- GitHub Codespaces demo environment for one-click evaluation of
  the MCP server in a browser-based development environment.

- One-command installers for Claude Code and Claude Desktop that
  automate binary download, configuration generation, and client
  registration.

- `--max-retries` flag for the kb-builder controls how many times
  transient embedding API errors are retried. The default is 5;
  set to 0 for unlimited retries. Backoff is capped at 60 seconds.

- Connection status field (`connected` or `unavailable`) in the
  `list_database_connections` tool response; databases with status
  `unavailable` are connected on demand when selected.

- Trace file logging for deep diagnostics of MCP interactions.
  Enable with `-trace-file <path>`, the `trace_file` configuration
  option, or the `PGEDGE_TRACE_FILE` environment variable. The
  server writes JSONL entries for tool calls, resource reads, prompt
  executions, HTTP requests, LLM interactions, database switches,
  configuration reloads, and session events.

### Internal

- Replaced the CGO SQLite driver with a pure Go driver, enabling
  fully static binaries without a C compiler dependency.

### Improved

- Expanded the configuration reference in `configuration.md` with
  database connection options (`allow_writes`, `allow_llm_switching`,
  `allowed_pl_languages`, pool settings, and access control), LLM
  proxy options, and previously undocumented CLI flags (`-debug`,
  `-db-*`, user management flags). Added missing entries to the
  environment variable reference and example configuration file.

- Comprehensive documentation expansion to improve Context7 benchmark
  coverage across all ten benchmark categories:

    - New `row-level-security.md` guide covering PostgreSQL RLS/CLS
      integration with the MCP server, including per-user database
      connections, session variable patterns, column-level security
      with grants and views, and a multi-tenant worked example.

    - New `distributed-deployment.md` guide covering multi-instance
      deployment with shared filesystem and object storage patterns,
      nginx and AWS ALB load balancer configuration, Docker Compose
      multi-instance example, Kubernetes deployment with ConfigMap
      and init containers, and knowledge base synchronization.

    - New `custom-knowledgebase-tutorial.md` with an end-to-end
      tutorial for building custom knowledge bases from domain
      documentation, including schema documentation patterns,
      business rules glossaries, KB builder configuration, and
      the internal SQLite database schema.

    - New `client-examples.md` with complete Python and JavaScript
      client implementations covering authentication, schema
      retrieval, query execution, database switching, TSV parsing,
      knowledgebase search, token lifecycle management, and error
      handling with automatic retry.

    - New `error-reference.md` documenting all HTTP status codes,
      JSON-RPC error codes, authentication errors, tool-specific
      errors, database access errors, and troubleshooting steps.

    - Expanded `claude_desktop.md` with a getting started guide,
      build instructions, YAML configuration examples, natural
      language query flow explanation, command-line flags reference,
      setup verification checklist, and detailed troubleshooting.

    - Expanded `authentication.md` with a database access control
      section documenting `available_to_users` authorization, a
      per-token database binding section, an authorization model
      summary, and a token lifecycle management section covering
      expiration detection, automatic re-authentication, and best
      practices for programmatic clients.

    - Expanded `api-reference.md` with schema retrieval examples
      including `curl` commands with authentication, TSV response
      parsing in Python and JavaScript, query execution examples
      with comprehensive error handling and retry logic, a query
      error reference table, and result format documentation.

    - Expanded `deploy_docker.md` with a complete consolidated
      `docker-compose.yml`, a full environment variable reference,
      a quick start guide, and Docker health check documentation.

    - Expanded `multiple_db_config.md` with Python and JavaScript
      client integration examples, access denied error handling,
      and a configuration settings reference clarifying the
      relationship between `llm_connection_selection` and
      `allow_llm_switching`.

### Fixed

- The server no longer exits when configured databases are
  unreachable at startup ([#82](https://github.com/pgEdge/pgedge-postgres-mcp/issues/82)).
  In STDIO mode, each database connection is now attempted
  independently; failures are logged as warnings and the server
  starts with whichever databases are reachable. Unreachable
  databases are connected on demand when a tool or the user
  selects them. The `list_database_connections` tool reports
  each database as `connected` or `unavailable`.

- Closed database clients are no longer returned from the client
  manager cache. Previously, background cleanup or database switching
  could close a client while tool registries still held a reference,
  causing intermittent "Connection pool not found" errors. Retrieval
  points now check a closed flag and transparently create a fresh
  client when needed.

- The `default_transaction_read_only` session parameter is now set
  with a `SET` command after connection instead of as a startup
  parameter. Connection poolers such as PgBouncer and HAProxy do
  not support arbitrary startup parameters; the previous approach
  caused connections to fail with an "unsupported startup parameter"
  error.

- Ollama embedding generation no longer retries or fails the entire
  batch when a chunk exceeds the model's context length. The builder
  detects the error immediately, progressively truncates the text at
  word boundaries (75 %, 50 %, 25 %), and skips the chunk with a
  warning if all attempts fail.

- Custom `pl-do` and `pl-func` tools no longer appear in `tools/list`
  when their language is not in `allowed_pl_languages`. Previously the
  language check only happened at execution time; the server now filters
  PL tools at registration time so clients only see tools they can use.

- Fixed PL/Perl custom tools (`pl-func` and `pl-do`) failing with
  "Unable to load JSON.pm into plperl" when using trusted `plperl`.
  Trusted `plperl` cannot load external Perl modules, so the wrapper
  now uses PostgreSQL's `jsonb_each_text()` via SPI to parse arguments
  instead of `JSON.pm`. Untrusted `plperlu` continues to use `JSON.pm`
  as before.

- Fixed Web GUI losing connection when switching between databases. The
  server now returns proper JSON error responses when the database is
  temporarily unavailable during switching, and the client handles these
  transient states gracefully with automatic retry logic instead of showing
  a disconnection error.

- SIGHUP configuration reload now invalidates stale database
  connections. Previously, reloading the configuration did not close
  connections whose parameters had changed, leaving the server with
  outdated connection settings until restart.

- The `-add-user`, `-add-token`, and related user/token management commands
  now respect the `user_file` and `token_file` paths from the server
  configuration file. Previously, these commands used hardcoded default paths
  regardless of the configuration, which could cause users or tokens to be
  added to the wrong file when custom paths were configured. The commands use
  the priority order: CLI flag > config file > default path. When `-user-file`
  or `-token-file` is explicitly provided on the command line, no configuration
  file is required (except for `-add-token` which needs database names from
  the config). This allows Docker containers and scripts to use these commands
  without a configuration file by specifying paths directly.

## [1.0.0-beta3] - 2026-01-21

### Added

#### Custom Tools

- New custom tools feature for defining database operations as callable MCP
  tools via YAML configuration
- Three tool types are supported:
    - `sql`: Execute parameterized SQL queries with `$1`, `$2`, etc.
      placeholders
    - `pl-do`: Execute PL/* DO blocks (anonymous functions) with automatic
      result handling via `set_config`/`current_setting`
    - `pl-func`: Create temporary PL/* functions with proper RETURN types
- Security controls via `allowed_pl_languages` configuration per database to
  restrict which procedural languages can be used
- Language support includes plpgsql, plpython3u, plv8, and plperl with
  automatic code wrapping and `mcp_return()` helper function
- Configurable per-tool timeout support
- Comprehensive validation of tool definitions at startup

#### LLM Database Connection Switching

- New `list_database_connections` tool allows LLMs to discover available
  database connections
- New `select_database_connection` tool allows LLMs to switch between databases
  during a conversation
- New `llm_connection_selection` configuration option to enable/disable the
  feature (disabled by default for security)
- New `allow_llm_switching` per-database option to exclude specific connections
  from LLM switching (defaults to true when feature is enabled)
- Real-time UI updates in web client when LLM switches databases
- CLI notification message when LLM switches databases

#### Prompt Argument Types

- Prompt arguments now support a `type` field with values `string` (default)
  or `boolean`
- Boolean arguments render as toggle switches in the web GUI instead of text
  fields
- Custom prompts in YAML can specify argument types for improved UI rendering

### Fixed

- The conversation history panel is now expanded by default when the web GUI
  loads, improving accessibility to past conversations.

- Fixed Web GUI database switching causing JSON parse error and disconnect loop.
  The `selectDatabase` function in `useDatabases.js` now checks `response.ok`
  before parsing the response as JSON; the auth middleware and database API
  handlers now return consistent JSON error responses instead of plain text.

- Improved login error messages in the web GUI. Authentication failures now
  display user-friendly messages like "Invalid username or password. Please
  try again." instead of technical RPC error codes.

- Standardized default configuration file paths for consistency. All config
  files now use the `postgres-mcp` prefix and search `/etc/pgedge/` first:
    - Config: `postgres-mcp.yaml` (previously `pgedge-postgres-mcp.yaml`)
    - Tokens: `postgres-mcp-tokens.yaml` (previously `pgedge-postgres-mcp-tokens.yaml`)
    - Users: `postgres-mcp-users.yaml` (previously `pgedge-postgres-mcp-users.yaml`)
    - Secret: `postgres-mcp.secret` (previously `pgedge-postgres-mcp.secret`)

- Improved error messages when the MCP server is unavailable. The web GUI now
  displays user-friendly messages for 502/503/504 errors instead of showing
  raw HTML error pages from the proxy.

- Fixed DDL and DML statements silently failing when `allow_writes` is enabled.
  The `query_database` tool now uses `tx.Exec()` for DDL (CREATE, DROP, ALTER,
  TRUNCATE) and DML (INSERT, UPDATE, DELETE) statements instead of `tx.Query()`,
  which could cause statements to not execute properly due to pgx's prepared
  statement caching behavior. DML statements with RETURNING clauses continue
  to use `tx.Query()` to capture returned rows.

## [1.0.0-beta2] - 2026-01-13

### Added

#### Write Access Mode

- New `allow_writes` configuration option for database connections
    - Disabled by default (read-only mode) for safety
    - When enabled, allows the LLM to execute DDL (CREATE, DROP, ALTER) and
      DML (INSERT, UPDATE, DELETE) statements
    - Automatic schema metadata refresh after DDL operations to keep
      `get_schema_info` results current
- Visual warnings for write-enabled databases:
    - Web client: Prominent amber warning banner when connected to a
      write-enabled database
    - Web client: Warning chip indicator in database selector popover
    - CLI: `[WRITE-ENABLED]` indicator in `/list databases` output
    - CLI: Warning message when switching to a write-enabled database
- Added `allow_writes` field to `pg://system_info` resource output
- Updated `query_database` tool description to dynamically indicate
  write access status

#### Token Management

- New `count_rows` tool for lightweight row counting before querying large
  tables
- Pagination support (`offset` parameter) in `query_database` tool for paging
  through large result sets
- Truncation detection in query results (fetches limit+1 rows to show "more
  data available" indicator)

#### Configuration Templates

- Added example configuration files in `examples/` directory:
    - `pgedge-postgres-mcp-http.yaml.example` - MCP server HTTP mode config
    - `pgedge-postgres-mcp-stdio.yaml.example` - MCP server stdio mode config
    - `pgedge-nla-cli-http.yaml.example` - CLI client HTTP mode config
    - `pgedge-nla-cli-stdio.yaml.example` - CLI client stdio mode config
    - `postgres-mcp-users.yaml.example` - User authentication template
    - `postgres-mcp-tokens.yaml.example` - Token authentication template

#### CLI Features

- Added `-mcp-server-config` command line flag for specifying the MCP server
  config file path in stdio mode

#### CI/CD

- Claude PR review GitHub Action workflow for automated code reviews
- CodeRabbit configuration for additional PR analysis

#### Knowledgebase Builder

- Hybrid chunking algorithm for improved RAG quality:

    - Two-pass algorithm: Pass 1 splits at semantic boundaries, Pass 2 merges
      undersized chunks
    - Structural element preservation: Code blocks, tables, lists, and
      blockquotes are kept intact when possible
    - Full heading hierarchy tracking: Chunks include breadcrumb context
      (e.g., "API Reference > Authentication > OAuth")
    - Smart splitting for oversized elements: Large code blocks split at line
      boundaries, tables at row boundaries, paragraphs at sentence boundaries
    - Chunk metadata now includes `HeadingPath` (full hierarchy) and
      `ElementTypes` (structural element types in chunk)

- Maintains Ollama compatibility with existing size limits (300 words / 3000
  chars)

### Changed

#### CLI Command Consistency

- Simplified LLM command names:
    - `/set llm-provider` → `/set provider`
    - `/set llm-model` → `/set model`
    - `/show llm-provider` → `/show provider`
    - `/show llm-model` → `/show model`
- Moved standalone listing commands under `/list`:
    - `/tools` → `/list tools`
    - `/resources` → `/list resources`
    - `/prompts` → `/list prompts`
- Added `/list providers` command to list available LLM providers
- Reorganized `/help` output into logical sections

#### Token Efficiency

- Query results now returned in TSV format instead of JSON for better token
  efficiency
- Custom SQL resource data returned in TSV format
- `get_schema_info` tool returns results in TSV format with additional relevant
  information and supports more targeted calls
- Removed redundant resource for retrieving schema info

#### Model Selection

- Model family matching when reloading saved conversations (handles
  date-suffixed model names like `claude-opus-4-5-20251101`)
- Web UI now uses family matching for model selection persistence
- CLI now restores database preference on load
- Added debug messages for model loading troubleshooting

#### Documentation

- Comprehensive documentation restructuring for online publication
- Added configuration setup instructions to README Web Client and CLI sections
- Added Quickstart guide
- Updated security documentation
- Added conversations API and database selection API documentation
- Fixed various documentation formatting issues and environment variable
  references

#### Docker

- Renamed `mcp-server` to `postgres-mcp` in Docker configuration (#12)

### Fixed

- CLI preference saving now works correctly
- Fixed test expecting wrong number of resources (1 instead of 2)
- Updated tests to expect 7 tools after count_rows addition
- Various typo fixes in documentation and configuration

## [1.0.0-beta1] - 2025-12-15

### Changed

This release marks the transition from alpha to beta status, indicating the
software is now feature-complete and ready for broader testing.

#### Internal

- Updated Claude Code configuration

## [1.0.0-alpha6] - 2025-12-12

### Added

#### CLI Features

- Added `none` authentication mode for CLI client to connect to servers with
  authentication disabled (`-mcp-auth-mode none`)

#### Knowledgebase

- Release workflow now builds kb.db with embeddings from all three providers
  (OpenAI, Voyage AI, Ollama)

### Changed

#### Naming

- Renamed server binary from `pgedge-postgres-mcp` to `pgedge-nla-svr`

#### Knowledgebase Builder

- Reduced chunk sizes to avoid hitting Ollama model token limits (250 words
  target, 300 max)
- Added character-based chunk limiting (3000 chars max) for technical content
  with high character-to-word ratios (XML/SGML)
- Improved markdown cleanup when building knowledgebase (removes images, link
  URLs, simplifies table borders)
- Added ASCII table border simplification to reduce token usage

### Fixed

- Fixed lint warnings in test files (unused types and unusedwrite warnings)
- Fixed tests that failed without database connection
- Fixed git branch handling when building knowledgebase (uses checkout -B to
  handle behind branches)
- Improved git pull handling when checking out branches for knowledgebase
  building

### Infrastructure

- Added Claude Code instructions file for development workflow

## [1.0.0-alpha5] - 2025-12-11

### Added

#### CLI Features

- Ability to cancel in-flight LLM queries by pressing Escape key
- Support for enabling/disabling colorization via configuration
- Terminal sanitization at startup to recover from broken terminal states

#### Web UI

- Login page animation

### Changed

#### Cross-Platform Compatibility

- Refactored syscall package usage into platform-specific files for proper
  cross-platform support (darwin, linux, windows)
- Improved terminal raw mode handling in escape key detection

#### UI Improvements

- Updated web UI styling to better match pgEdge Cloud design
- Removed unnecessary checks for LLM environment variables

### Fixed

#### Critical Bug Fixes

- **CLI Output Bug**: Fixed staircase indentation issue where CLI output
  progressively indented to the right
- **Terminal State**: Fixed terminal being left in broken state after CLI
  exit due to raw mode not being properly restored
- **Compaction Bug**: Fixed tool_use and tool_result messages being
  separated during conversation compaction, which caused Anthropic API
  errors (400 Bad Request with "tool_use_id not found")

#### Other Fixes

- Fixed first load of a conversation not displaying correctly in web UI
- Fixed broken documentation URLs in README after docs restructuring

### Infrastructure

- GitHub Actions workflow improvements:
    - Build kb.db using goreleaser
    - Include kb.db in kb-builder archive
    - Use token to pull private repos
    - Create bin directory before using it in release workflow
    - Fix build command issues
    - Fix dirty git state error in workflow
    - Use architecture-specific runners for release builds

## [1.0.0-alpha4] - 2025-12-08

### Added

#### Conversation History

- Server-side conversation storage using SQLite database for persistent
  chat history
- REST API endpoints for conversation CRUD operations
  (`/api/conversations/*`)
- Web client conversation panel with list, load, rename, and delete
  functionality
- CLI conversation history commands (`/history`, `/new`, `/save`) when
  running in HTTP mode with authentication
- Automatic provider/model restoration when loading saved conversations
- Database connection tracking per conversation
- History replay with muted colors when loading CLI conversations
- Auto-save behavior in web client after first assistant response

#### Configuration

- Configuration options to selectively enable/disable built-in tools,
  resources, and prompts via the `builtins` section in the config file
- Disabled features are not advertised to the LLM and return errors if
  called directly
- The `read_resource` tool is always enabled as it's required for listing
  resources

#### LLM Provider Improvements

- Dynamic model retrieval for Anthropic provider - available models are
  now fetched from the API instead of being hardcoded
- Display client and server version numbers in CLI startup banner

#### Build & Release

- GitHub Actions workflow for automated release artifact generation
  using goreleaser
- Local verification script for goreleaser artifacts

## [1.0.0-alpha3] - 2025-12-03

### Added

- Web client documentation with screenshots demonstrating all UI features
- Documentation comparing RAG (Retrieval-Augmented Generation) and MCP
  approaches
- Optional Docker container variant with pre-built knowledgebase database
  included

### Changed

#### Naming

- Renamed the server to *pgEdge MCP Server* (from *pgEdge NLA Server*)

#### Knowledgebase System

- `search_knowledgebase` tool now accepts arrays for product and version
  filters, allowing searches across multiple products/versions in a single
  query
- Parameter names changed from `project_name`/`project_version` to
  `project_names`/`project_versions` (arrays of strings)
- Added `list_products` parameter to discover available products and versions
  before searching
- Improved `search_knowledgebase` tool prompt with:
    - Critical warning about exact product name matching at the top
    - Step-by-step workflow guidance (discover products first, then search)
    - Troubleshooting section for zero-result scenarios
    - Updated examples showing realistic product names

### Fixed

- Docker Compose health check now uses correctly renamed binary

## [1.0.0-alpha2] - 2025-11-27

### Added

#### Token Usage Optimization

- Smart auto-summary mode for `get_schema_info` tool when database has >10
  tables
- New `compact` parameter for `get_schema_info` to return minimal output
  (table names + column names only)
- Token estimation and tracking for individual tool calls (visible in debug
  mode)
- Resource URI display in activity log for `read_resource` calls
- Proactive compaction triggered by token count threshold (15,000 tokens)
- Rate limit handling with automatic 60-second pause and retry

#### Prompt Improvements

- Added `<fresh_data_required>` guidance to prompts to prevent LLM from
  using stale information when database state may have changed
- Updated `explore-database` prompt with rate limit awareness and tool
  call budget guidance
- Enhanced prompts guide LLMs to minimize tool calls for token efficiency

#### Multiple Database Support

- Configure multiple PostgreSQL database connections with unique names
- Per-user access control via `available_to_users` configuration field
- Automatic default database selection based on user accessibility
- Runtime database switching in both CLI and Web clients
- Database selection persistence across sessions via user preferences
- CLI commands: `/list databases`, `/show database`, `/set database <name>`
- Web UI database selector in status banner with connection details
- Database switching disabled during LLM query processing to prevent
  data consistency issues
- Improved error messages when no databases are accessible to a user
- API token database binding via `-token-database` flag or interactive
  prompt during token creation

#### Knowledgebase System

- Complete knowledgebase system with SQLite backend for offline
  documentation search
- `search_knowledgebase` MCP tool for semantic similarity search across
  pre-built documentation
- KB builder utility for creating knowledgebase from markdown, HTML,
  SGML, and DocBook XML sources
- Support for multiple embedding providers (Voyage AI, OpenAI, Ollama)
  in knowledgebase
- Project name and version filtering for targeted documentation search
- Independent API key configuration for knowledgebase (separate from
  embedding and LLM sections)
- DocBook XML format support for PostGIS and similar documentation
- Optional project version field in documentation sources

#### LLM Provider Management

- Dynamic Ollama model selection with automatic fallback to available
  models
- Per-provider model persistence in CLI (remembers last-used model for
  each provider)
- Per-provider model persistence in Web UI (using localStorage)
- Automatic preference validation and sanitization on load
- Default provider priority order (Anthropic → OpenAI → Ollama)
- Preferred Ollama models list with tool-calling support verification
- Runtime model validation against provider APIs before selection
- Provider selection now validates that provider is actually configured
- Filtered out Claude Opus models from Anthropic (causes tool-calling
  errors)
- Filtered out embedding, audio, and image models from OpenAI model list

#### Security & Authentication

- Rate limiting for failed authentication attempts (configurable window
  and max attempts)
- Account lockout after repeated failed login attempts
- Per-IP rate limiting to prevent brute force attacks

#### Tools, Resources, and Prompts

- Support for custom user-defined prompts in
  `examples/pgedge-postgres-mcp-custom.yaml`
- Support for custom user-defined resources in custom definitions file
- New `execute_explain` tool for query performance analysis
- Enhanced tool descriptions with usage examples and best practices
- Added a schema-design prompt for helping design database schemas

### Changed

#### Naming & Organization

- Renamed the project to *pgEdge Natural Language Agent*
- Renamed all binaries and configuration files for consistency:
    - Server: `pgedge-pg-mcp-svr` -> `pgedge-postgres-mcp`
    - CLI: `pgedge-pg-mcp-cli` -> `pgedge-nla-cli`
    - Web UI: `pgedge-mcp-web` -> `pgedge-nla-web`
    - KB Builder: `kb-builder` -> `pgedge-nla-kb-builder`
- Default server configuration files now use `pgedge-postgres-mcp-*.yaml` naming
- Default CLI configuration files now uses `pgedge-nla-cli.yaml` naming
- Custom definitions file: `pgedge-postgres-mcp-custom.yaml`
- Updated all documentation and examples to reflect new naming

#### Configuration

- Reduced default similarity_search token budget from 2500 to 1000
- Default OpenAI model changed from `gpt-5-main` to `gpt-5.1`
- Independent API key configuration for knowledgebase, embedding, and
  LLM sections
- Support for KB-specific environment variables:
  `PGEDGE_KB_VOYAGE_API_KEY`, `PGEDGE_KB_OPENAI_API_KEY`

#### UI/UX Improvements

- Enhanced LLM system prompts for better tool usage guidance
- CLI now saves current model when switching providers
- Web UI correctly remembers per-provider model selections
- Improved error messages and warnings for invalid configurations
- CLI `/list tools`, `/list resources`, and `/list prompts` commands now
  sort output alphabetically
- Web UI favicon added
- Web UI: Moved Clear button from floating position to bottom toolbar
  (next to Settings)
- Web UI: Added Save Chat button to export conversation history as
  Markdown
- Web UI: Improved light mode contrast with gray page background for
  paper effect

### Fixed

- **Critical**: Fixed Voyage AI API response parsing (was expecting flat
  `embedding` field, actual API returns `data[].embedding`)
- **Security**: Custom HTTP handlers (`/api/chat/compact`, `/api/llm/chat`)
  now require authentication when auth is enabled (provider/model listing
  endpoints remain public for login page)
- CLI no longer randomly switches to wrong provider/model on startup
- Invalid provider/model combinations in preferences now automatically
  corrected with warnings
- Web UI model selection now persists correctly across provider switches
- Applied consistent code formatting with `gofmt`
- Removed unused kb-dedup utility
- Fixed gocritic lint warnings
- Fixed data race in rate limiter tests

### Infrastructure

- Docker images updated to Go 1.24
- CI/CD workflows upgraded to Go 1.24 with PostgreSQL 18 testing support
- Start scripts refactored with variable references for improved
  maintainability

## [1.0.0-alpha1] - 2025-11-21

### Added

#### Core Features

- Model Context Protocol (MCP) server implementation
- PostgreSQL database connectivity with read-only transaction
  enforcement
- Support for stdio and HTTP/HTTPS transport modes
- TLS support with certificate and key configuration
- Hot-reload capability for authentication files (tokens and users)
- Automatic detection and handling of configuration file changes

#### MCP Tools (5)

- `query_database` - Execute SQL queries in read-only transactions
- `get_schema_info` - Retrieve database schema information
- `hybrid_search` - Advanced search combining BM25 and MMR algorithms
- `generate_embeddings` - Create vector embeddings for semantic search
- `read_resource` - Access MCP resources programmatically

#### MCP Resources (3)

- `pg://stat/activity` - Current database connections and activity
- `pg://stat/database` - Database-level statistics
- `pg://version` - PostgreSQL version information

#### MCP Prompts (3)

- Semantic search setup workflow
- Database exploration guide
- Query diagnostics helper

#### CLI Client

- Production-ready command-line chat interface
- Support for multiple LLM providers (Anthropic, OpenAI, Ollama)
- Anthropic prompt caching (90% cost reduction)
- Dual mode support (stdio subprocess or HTTP API)
- Persistent command history with readline support
- Bash-like Ctrl-R reverse incremental search
- Runtime configuration with slash commands
- User preferences persistence
- Debug mode with LLM token usage logging
- PostgreSQL-themed UI with animations

#### Web Client

- Modern React-based web interface
- AI-powered chat for natural language database interaction
- Real-time PostgreSQL system information display
- Light/dark theme support with system preference detection
- Responsive design for desktop and mobile
- Token usage display for LLM interactions
- Chat history with prefix-based search
- Message persistence and state management
- Debug mode with toggle in preferences popover
- Markdown rendering for formatted responses
- Inline code block rendering
- Auto-scroll with smart positioning

#### Authentication & Security

- Token-based authentication with SHA256 hashing
- User-based authentication with password hashing
- API token management with expiration support
- File permission enforcement (0600 for sensitive files)
- Per-token connection isolation
- Input validation and sanitization
- Secure password storage in `.pgpass` files
- TLS/HTTPS support for encrypted communications

#### Docker Support

- Complete Docker Compose deployment configuration
- Multi-stage Docker builds for optimized images
- Container health checks
- Volume management for persistent data
- Environment-based configuration
- CI/CD pipeline for Docker builds

#### Infrastructure

- Comprehensive CI/CD with GitHub Actions
- Automated testing for server, CLI client, and web client
- Docker build and deployment validation
- Documentation build verification
- Code linting and formatting checks
- Integration tests with real PostgreSQL databases

#### LLM Proxy

- JSON-RPC proxy for LLM interactions from web clients
- Support for multiple LLM providers
- Request/response logging
- Error handling and status reporting
- Dynamic model name loading for Anthropic
- Improved tool call parsing for Ollama

### Documentation

- Comprehensive user guide covering all features
- Configuration examples for server, tokens, and clients
- API reference documentation
- Architecture and internal design documentation
- Security best practices guide
- Troubleshooting guide with common issues
- Docker deployment guide
- Building chat clients tutorial with Python examples
- Query examples demonstrating common use cases
- CI/CD pipeline documentation
- Testing guide for contributors

[Unreleased]: https://github.com/pgEdge/pgedge-nla/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/pgEdge/pgedge-nla/compare/v1.0.0-beta3...v1.0.0
[1.0.0-beta3]: https://github.com/pgEdge/pgedge-nla/releases/tag/v1.0.0-beta3
[1.0.0-beta2]: https://github.com/pgEdge/pgedge-nla/releases/tag/v1.0.0-beta2
[1.0.0-beta1]: https://github.com/pgEdge/pgedge-nla/releases/tag/v1.0.0-beta1
[1.0.0-alpha6]: https://github.com/pgEdge/pgedge-nla/releases/tag/v1.0.0-alpha6
[1.0.0-alpha5]: https://github.com/pgEdge/pgedge-nla/releases/tag/v1.0.0-alpha5
[1.0.0-alpha4]: https://github.com/pgEdge/pgedge-postgres-mcp/releases/tag/v1.0.0-alpha4
[1.0.0-alpha3]: https://github.com/pgEdge/pgedge-postgres-mcp/releases/tag/v1.0.0-alpha3
[1.0.0-alpha2]: https://github.com/pgEdge/pgedge-postgres-mcp/releases/tag/v1.0.0-alpha2
[1.0.0-alpha1]: https://github.com/pgEdge/pgedge-postgres-mcp/releases/tag/v1.0.0-alpha1
