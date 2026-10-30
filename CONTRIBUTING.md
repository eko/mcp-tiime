# Contributing

Thanks for your interest in `mcp-tiime`! Bug reports, new tools, fixes after a Tiime change and
documentation improvements are all welcome.

> This project drives a real accounting account through an undocumented API. Two rules matter
> more than anything else here: **never share credentials, tokens or personal financial data**
> (in issues, pull requests, logs or test fixtures), and **never make an irreversible action
> easy to trigger by mistake**.

## Contents

- [Ways to contribute](#ways-to-contribute)
- [Development setup](#development-setup)
- [Project layout](#project-layout)
- [Adding a tool](#adding-a-tool)
- [When Tiime changes its API](#when-tiime-changes-its-api)
- [Write safety](#write-safety)
- [Code conventions](#code-conventions)
- [Testing](#testing)
- [Commits and pull requests](#commits-and-pull-requests)
- [Releases](#releases)

## Ways to contribute

- **Report a bug**: open an issue with the tool name, the arguments and the error message.
  Redact amounts, IBANs, names and IDs you do not want to share.
- **Request a tool**: describe what you do in the Tiime web application today. The network
  request it makes (route and payload, redacted) helps a lot.
- **Fix or add a tool**: see below. For a large change, open an issue first so we can agree on
  the approach.
- **Security issue**: do not open a public issue, see [SECURITY.md](SECURITY.md).

## Development setup

Requirements: Go 1.27 or later, and [golangci-lint](https://golangci-lint.run) v2.15 or later.

```bash
git clone https://github.com/eko/mcp-tiime.git
cd mcp-tiime

go build -o bin/mcp-tiime ./cmd/mcp-tiime
go test -race ./...
golangci-lint run ./...
```

To try your build in Claude Code:

```bash
claude mcp add tiime-dev -- "$PWD/bin/mcp-tiime"
```

After rebuilding, reconnect it with `/mcp` → **tiime-dev** → **Reconnect**. Other clients are
covered in [docs/mcp-clients.md](docs/mcp-clients.md).

## Project layout

| Directory | Purpose |
|---|---|
| `cmd/mcp-tiime` | Entry point, `login` and `check` commands, `uber-go/fx` wiring |
| `internal/config` | Configuration with `viper` (`TIIME_*` variables) |
| `internal/tiime` | Tiime API HTTP client, Auth0 authentication (MFA, PKCE) |
| `internal/tools` | MCP tools, one interface and one file per domain; `expert*.go` for the expert mode |
| `docs/api-tiime.md` | Reverse-engineering notes on the Tiime API: routes, headers, formats, pitfalls |
| `mcpb/` | Claude Desktop extension manifest |
| `scripts/` | Release helpers |

## Adding a tool

1. **Client method** in `internal/tiime`: one method per Tiime call, in the file of its domain
   (`banking.go`, `invoicing.go`…). Shared types live in `types.go`, types used by a single
   domain next to its methods.
2. **Domain interface** in `internal/tools`: declare the method in the domain interface
   (`BankingAPI`, `VATAPI`…). Tools depend on these small interfaces, never on the concrete
   client, so they can be tested with fakes.
3. **Tool registration** in the domain `register…` function:
   - a `snake_case` name starting with a verb (`list_`, `get_`, `create_`, `update_`…);
   - a description written for the agent: what the tool does, when to use it, and what it
     does **not** do;
   - annotations: `ReadOnlyHint` for reads, `DestructiveHint` for irreversible writes;
   - an input struct with `json` and `jsonschema` tags. Lists take the shared `PageInput`
     (`offset`, `limit` ≤ 100) and return `has_more`; company-scoped tools take
     `CompanyInput`.
4. **Validation**: reject inconsistent input before calling Tiime (negative amount, unknown
   status, relative file path…).
5. **Tests**: a client test against `httptest` (route, method, headers, payload) and a tool
   test with the fake API.
6. **Tool list**: add the name to the expected list of `TestRegisterListsTools`.
7. **Docs**: add the tool to the README tables, with its **Tested** status (✅ validated on a
   real account, 🟡 read-only validated, ⬜ never run for real), and document the route in
   `docs/api-tiime.md`.

## When Tiime changes its API

Tiime does not publish its API: routes come from the code of the `apps.tiime.fr` web
application. `docs/api-tiime.md` explains how to find a route in a new version of the
application. When you fix a route:

- update `docs/api-tiime.md` with what changed and the application version
  (`TIIME_APP_VERSION`) you checked against;
- bump the default `app_version` in `internal/config` if the application version changed;
- say in the pull request what you checked on a real account, and with which tool.

## Write safety

Changes that write to Tiime get a careful review:

- Mark irreversible tools with `DestructiveHint` so MCP clients ask for confirmation.
- When Tiime replaces a whole set (documents or invoices matched with a transaction), read the
  current set and merge it: a tool must never drop data it was not asked to drop.
- Transfers from the Tiime business account and sending or cancelling VAT returns are
  **deliberately out of scope**. A pull request adding them needs a dedicated safeguard
  discussed in an issue first.

## Code conventions

- `gofmt`, `go vet` and `golangci-lint run ./...` must pass: the CI runs them.
- Small, focused functions and interfaces; dependencies injected through constructors and wired
  with `uber-go/fx` in `cmd/mcp-tiime`.
- Errors are wrapped with context (`fmt.Errorf("listing invoices: %w", err)`); use sentinel
  errors and `errors.Is` / `errors.As` instead of comparing strings.
- Logs go through `log/slog` on **stderr**: stdout carries the MCP protocol and nothing else may
  write to it.
- Functions with more than two parameters put one parameter per line, as in the rest of the
  code.
- Comments explain *why*, not *what*. Exported symbols have a doc comment.

## Testing

- Table-driven tests with [testify](https://github.com/stretchr/testify) (`require` for
  preconditions, `assert` for checks).
- HTTP calls are tested against `httptest` servers: tests never reach the real Tiime API.
- Fixtures are made up or fully anonymized. No real IBAN, amount, name, company ID or token.
- Run the whole suite with the race detector before opening a pull request:
  `go test -race ./...`.

## Commits and pull requests

- Use [Conventional Commits](https://www.conventionalcommits.org): `feat:`, `fix:`, `docs:`,
  `test:`, `ci:`, `refactor:`, `chore:`. The release changelog is generated from them, and
  `docs:`, `test:` and `ci:` commits are left out of it.
- One topic per pull request. Fill in the template: what changed, how it was tested, and
  whether it was checked on a real account.
- Update the README and the docs in the same pull request as the code.

## Releases

Releases are cut by the maintainer by pushing a `vX.Y.Z` tag. The release workflow then
publishes:

- the binaries for Linux, macOS and Windows (amd64 and arm64) and `checksums.txt`;
- the Claude Desktop extension `mcp-tiime.mcpb`;
- the Docker image `ghcr.io/eko/mcp-tiime`.

Tags with a suffix (`v1.2.0-rc.1`) are published as pre-releases.

By contributing, you agree that your contributions are licensed under the
[MIT License](LICENSE).
