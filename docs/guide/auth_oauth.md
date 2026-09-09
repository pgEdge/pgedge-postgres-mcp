# Authentication - OAuth

The MCP server includes a built-in OAuth 2.0 authorisation server. It
lets a client sign a user in through a branded login page instead of
handling a password or an API token itself, and it issues short-lived
access tokens backed by the same user accounts described in
[Authentication - User Management](auth_user.md).

## When to Use OAuth

Use OAuth wherever a client can open a login page and does not want to
store a long-lived credential of its own.

- Claude Desktop and the Claude mobile apps use OAuth when the server
  is added as a remote connector.
- The bundled CLI chat client uses OAuth by default in HTTP mode.
- The bundled web client uses OAuth when the server advertises it,
  falling back to username and password otherwise.
- Any other MCP client that supports OAuth discovery can use it too.

Machine-to-machine callers that cannot open a browser should continue
to use an [API token](auth_token.md) instead.

## How the Flow Works

A client first fetches the server's metadata from
`/.well-known/oauth-authorization-server`, then either registers itself
dynamically or uses a preconfigured client ID. It sends the user's
browser to `/oauth/authorize` with a PKCE code challenge; the server
renders the branded login page, the user enters a username and
password, and the server redirects back to the client's redirect URI
with an authorisation code. The client exchanges that code, along with
the matching PKCE verifier, for an access token and a refresh token at
`/oauth/token`. A client with no browser, such as a headless CLI
session, uses the device authorisation grant instead: it polls
`/oauth/token` while the user completes the same login page on another
device.

## Enabling OAuth

OAuth becomes active once authentication is enabled, the `oauth` method
has not been disabled, and an issuer URL is configured. The issuer is
the base URL clients use to discover and reach the authorisation
server; it usually matches the server's own public address.

In the following example, the `oauth.issuer` setting turns on the
authorisation server:

```yaml
http:
    auth:
        enabled: true
        oauth:
            issuer: "https://mcp.example.com"
```

The issuer must be an absolute `http` or `https` URL with no query
string or fragment, and must use `https` unless the host is `localhost`
or a loopback address. Refer to
[Specifying your Configuration Preferences](configuration.md) for the
complete set of `http.auth.oauth` options and their defaults.

## Method Toggles

Each authentication method can be switched off independently under
`http.auth.methods`, leaving the other methods untouched.

```yaml
http:
    auth:
        methods:
            api_tokens: true
            password_login: true
            oauth: true
```

Every method defaults to `true`, so an existing configuration that
never mentions `methods` keeps working exactly as before. At least one
method must remain enabled while authentication itself is enabled.

## Startup Output and Discovery

With OAuth active, the server logs its issuer at startup, and adds the
issuer's own origin to the browser origins it accepts:

```text
Accepting browser requests from: loopback origins only (localhost, 127.0.0.1, ::1; any port)
OAuth authorisation server enabled, issuer http://localhost:8099
Starting MCP server in HTTP mode on 127.0.0.1:8099
Accepting browser requests from the OAuth issuer origin: http://localhost:8099
```

A client discovers the authorisation server by fetching
`/.well-known/oauth-authorization-server`, which returns a document
such as the following:

```json
{
    "issuer": "http://localhost:8099",
    "authorization_endpoint": "http://localhost:8099/oauth/authorize",
    "token_endpoint": "http://localhost:8099/oauth/token",
    "registration_endpoint": "http://localhost:8099/oauth/register",
    "device_authorization_endpoint": "http://localhost:8099/oauth/device",
    "revocation_endpoint": "http://localhost:8099/oauth/revoke",
    "response_types_supported": ["code"],
    "grant_types_supported": [
        "authorization_code",
        "refresh_token",
        "urn:ietf:params:oauth:grant-type:device_code"
    ],
    "code_challenge_methods_supported": ["S256"],
    "token_endpoint_auth_methods_supported": ["none"],
    "scopes_supported": ["mcp"]
}
```

A request to a protected endpoint without a valid access token
receives a `401` response carrying a `WWW-Authenticate` header that
points the client at the protected resource metadata:

```text
WWW-Authenticate: Bearer resource_metadata="http://localhost:8099/.well-known/oauth-protected-resource"
```

## Claude Desktop and Claude Mobile

Add the server as a remote connector rather than a local `stdio`
server. In Claude Desktop, open Settings, choose Connectors, select Add
custom connector, and enter the server's URL. Claude Desktop opens the
server's login page in a browser; sign in there to complete the
connection. The Claude mobile apps share the same connector list once
it exists, so a connector added on the desktop appears on mobile
without repeating the steps. See
[Configuring the Server for use with Claude Desktop](claude_desktop.md)
for the `stdio` route and this remote route side by side.

## CLI Behaviour

The CLI chat client's `auth_mode` setting accepts `auto`, `none`,
`token`, `user` and `oauth`. The default, `auto`, tries OAuth first and
falls back to token or username and password authentication when the
server does not advertise OAuth.

By default, the CLI opens a browser and completes the authorisation
code grant through a loopback redirect. Pass `--no-browser`, or set
`no_browser: true` in the CLI configuration file, to use the device
authorisation grant instead; the CLI then prints a URL and a code for
the user to enter on another device. The `/logout` slash command
revokes the current OAuth session and clears its cached tokens.

The CLI caches OAuth tokens in a file named `oauth-tokens.yaml`,
stored beside its preferences file, and refreshes them automatically
as they approach expiry.

In the following example, the `-mcp-auth-mode` flag and `-no-browser`
flag select the device authorisation grant:

```bash
./bin/pgedge-nla-cli -mcp-auth-mode oauth -no-browser \
    -mcp-url https://mcp.example.com
```

## Web Client Behaviour

The bundled web client discovers OAuth from the server's metadata and,
when available, offers it as a sign-in option alongside username and
password. Signing in redirects the browser to the server's login page
and back to the web client's own origin.

The web client's origin must appear in `http.allowed_origins`, because
the server derives the OAuth redirect URI it accepts,
`<origin>/oauth/callback`, from each origin in that list. See
[Browser Origins](configuration.md#browser-origins) for the setting.

```yaml
http:
    allowed_origins:
        - https://mcp.example.com
```

## Branding

The login page's appearance comes from `http.auth.oauth.login_page`.
Every field is optional and falls back to the default shown below.

| Field | Description | Default |
|-------|--------------|---------|
| `title` | Form heading. | `Sign in` |
| `subtitle` | Text below the title. | See below |
| `message` | Notice paragraphs; a blank line starts a new one. | (none) |
| `footer` | Text below the form. | (none) |
| `logo_file` | Path to a custom logo, replacing the built-in one. | Built-in logo |
| `primary_colour` | CSS hex colour for buttons. | `#15AABF` |
| `secondary_colour` | CSS hex colour paired with the primary one. | `#0C8599` |
| `template_file` | Path to a custom template, replacing the built-in page. | Built-in page |

The `subtitle` field defaults to
`Sign in to the pgEdge Postgres MCP Server`.

In the following example, the `login_page` block sets a custom title
and colour scheme:

```yaml
http:
    auth:
        oauth:
            login_page:
                title: "Example Corp"
                subtitle: "Sign in to your Example Corp database assistant"
                message: |
                    Use your Example Corp single sign-on username and
                    password.
                footer: "(c) Example Corp"
                logo_file: "/etc/pgedge/logo.png"
                primary_colour: "#123456"
                secondary_colour: "#654321"
```

## Custom Templates

Set `template_file` to replace the login page entirely with a custom
HTML template. See `examples/oauth/login.html` for an annotated copy of
the built-in template to start from, and
`examples/oauth/README.md` for a short explanation of the contract.

A custom template must define a template named `page`, and its form
must post back to the same URL with these hidden fields plus the
CSRF field:

- `csrf_token`, the anti-forgery token supplied in `.CSRFToken`.
- `response_type`, `client_id`, `redirect_uri`, `scope`, `state`,
  `code_challenge` and `code_challenge_method`, taken from `.OAuth`.

The template also receives `.Error`, a message to show when a previous
attempt failed, `.Client`, the requesting client's name where known,
`.UserCode` and `.IsDeviceFlow` for the device authorisation grant, and
`.Page`, one of `login`, `device` or `done`, which the template can use
to vary its layout for each stage of the flow.

## Security Notes

The login page never renders the credential itself back to the client;
only the authorisation code and, later, the access and refresh tokens
leave the server. Refresh and access tokens follow the lifetimes
configured under `http.auth.oauth`, and the token endpoint rate-limits
repeated failed grants per client IP address, the same limiter used
for password login. Serve the issuer over `https` in any deployment
reachable from outside the machine it runs on; the server refuses a
plain `http` issuer for any host other than `localhost` or a loopback
address.

## Troubleshooting

If a client cannot discover OAuth, confirm that
`/.well-known/oauth-authorization-server` returns a JSON document
rather than a `404`; a `404` means `oauth.issuer` is unset, the
`oauth` method is disabled, or authentication itself is disabled.

If sign-in completes but the client never receives its token, check
that the client's redirect URI appears in
`http.auth.oauth.allowed_redirect_uris`, or that the web client's
origin appears in `http.allowed_origins` so the server can derive it
automatically.

If a browser refuses to post the login form, or a request to
`/oauth/authorize` is rejected before the login page even renders,
check the server's Origin handling; see [Browser Origins and DNS
Rebinding](security.md#browser-origins-and-dns-rebinding).

If the CLI or web client reports the same failure only in headless
environments, use the device authorisation grant: pass `--no-browser`
to the CLI, or complete the code shown on another device.
