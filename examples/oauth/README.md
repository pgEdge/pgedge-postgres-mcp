# OAuth Login Page Examples

This directory contains a starting point for customising the OAuth
login page that the pgEdge Postgres MCP Server renders during
sign-in.

## Files

This directory provides the following files.

- `login.html` is an annotated copy of the server's built-in login
  page template, with a comment header explaining the template
  contract that any custom template must follow.
- `config.yaml` is a complete `http.auth` fragment showing OAuth
  enabled alongside branding settings.

## Using a Custom Template

Copy `login.html` to a location the server can read, remove or edit
the parts you want to change, and point the server at it:

```yaml
http:
    auth:
        oauth:
            issuer: "https://mcp.example.com"
            login_page:
                template_file: "/etc/pgedge/login.html"
```

Read the comment header in `login.html` before editing, since it
sets out every hidden field and data value the server expects the
template to use. See [Authentication -
OAuth](../../docs/guide/auth_oauth.md#custom-templates) for the full
explanation.

## Using the Configuration Fragment

Merge the relevant parts of `config.yaml` into your own server
configuration file, replacing the example hostname and colours with
your own values. See [Authentication -
OAuth](../../docs/guide/auth_oauth.md) for every available setting.
