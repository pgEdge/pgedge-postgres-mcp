# TO-DO Items

*Note:* These items are ideas for future enhancements. Their suitability needs
to be assessed as part of the initial work, and inclusion here does not 
represent a roadmap or any guarantee that any features will actually be
implemented.

## LLM Support

-  Add support for use of Google Gemini as the LLM provider.
-  Add support for use of OpenAI API-compatible local LLM providers, such as
    LM Studio, Docker Model Runner, and EXO. These should just work if 
    configured as OpenAI, but without a requirement for an API key.
- Add support for arbitrary request headers to be added to LLM request calls
    to support servers such as Portkey which requires addition of headers such
    as `x-portkey-provider: openai`

## Tools

- Add an MCP tool for validating SQL queries before presentation to the user.

## Security

- Add support for OAuth2 authentication, to allow use as a Claude Custom 
    Connector.
- Add support for Enterprise authentication, e.g. LDAP/AD (Kerberos?).

## Management

- Add REST APIs to allow automated management of users and connections (store
    in SQLite, merge with hard-configured users/connections).
