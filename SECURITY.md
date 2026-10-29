# Security policy

`mcp-tiime` handles Tiime credentials and a session that grants access to accounting and bank
data. Security reports are taken seriously.

## Reporting a vulnerability

**Do not open a public issue.** Report it privately through
[GitHub private vulnerability reporting](https://github.com/eko/mcp-tiime/security/advisories/new).

Please include:

- what an attacker can do, and under which conditions;
- the steps to reproduce, with the version (`mcp-tiime` server version, or the release tag);
- any idea of a fix.

Never include real credentials, tokens, IBANs or account data in a report: describe them or use
made-up values.

You should get a first answer within a week. Once a fix is released, the advisory is published
with credit to the reporter, unless you prefer to stay anonymous.

## Supported versions

Only the latest release receives security fixes.

## Scope

In scope, for instance:

- leaks of the password, the access token or the refresh token (logs, error messages, files
  with loose permissions, tool results);
- a tool that can be made to act on another company or account than intended;
- a way to bypass the input validation or the merge of matched documents, leading to data loss;
- path handling in `upload_document` or downloads that reads or writes outside the intended
  files.

Out of scope:

- vulnerabilities of Tiime itself: report them to Tiime;
- an agent following instructions it was given by the user who configured it.

## Good practices for users

- Treat the session cache (`~/.config/mcp-tiime/token.json`, `0600`) as a secret: it grants
  access to the Tiime account. Delete it to revoke the local session.
- Prefer a client that stores the password in the system keychain (Claude Desktop extension,
  VS Code inputs) over a plain-text config file.
- Keep the MCP client's confirmation prompt on for destructive tools.
