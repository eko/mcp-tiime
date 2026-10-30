# mcp-tiime

A Go [MCP](https://modelcontextprotocol.io) server to drive [Tiime](https://www.tiime.fr) from
Claude or any MCP agent: invoicing, banking, receipts, expense reports, VAT and exchanges with
the accounting firm. **92 tools**, MFA login included. Accounting firms using Tiime Expert can
switch to an experimental [expert mode](docs/expert-mode.md) (43 tools across their portfolio).

> ⚠️ Tiime does not publish an API. This server uses the **internal** API of the
> `apps.tiime.fr` web application (`chronos-api.tiime-apps.com/v1`). Routes and formats were
> extracted from the web application code and checked against a real account, but nothing is
> guaranteed: Tiime may change them without notice. This project is not affiliated with Tiime.

## Contents

- [Installation](#installation)
- [Quick start](#quick-start)
- [Example requests](#example-requests)
- [Tools](#tools)
- [Accounting firms (expert mode)](#accounting-firms-expert-mode)
- [Write safety](#write-safety)
- [Authentication and MFA](#authentication-and-mfa)
- [Configuration](#configuration)
- [Behaviors to know](#behaviors-to-know)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

## Installation

### Claude Desktop and Cowork (one-click)

Download [`mcp-tiime.mcpb`](https://github.com/eko/mcp-tiime/releases/latest/download/mcp-tiime.mcpb)
from the latest release and double-click it: Claude Desktop installs the extension and asks for
your Tiime email and password. It works on macOS and Windows, in chats and in Cowork. See the
[Claude Desktop extension guide](docs/claude-desktop-extension.md).

### Prebuilt binaries

Each [release](https://github.com/eko/mcp-tiime/releases) ships binaries for Linux, macOS and
Windows (amd64 and arm64), with a `checksums.txt` file.

```bash
tar -xzf mcp-tiime_<version>_darwin_arm64.tar.gz
./mcp-tiime login
```

### Docker

A multi-arch image (`linux/amd64`, `linux/arm64`) is published on GitHub Container Registry.
The session cache and downloads live in `/data`, so keep it in a volume:

```bash
# First login, interactive
docker run --rm -it -v mcp-tiime:/data ghcr.io/eko/mcp-tiime login

# MCP server over stdio
claude mcp add tiime -- docker run --rm -i -v mcp-tiime:/data ghcr.io/eko/mcp-tiime
```

`upload_document` reads files from the container: mount the folder that holds your receipts
(for instance `-v "$HOME/Documents:$HOME/Documents:ro"`) and pass absolute paths. Mount a host
folder on `/data/downloads` to get the downloaded PDFs on your machine.

### From source

Requirements: Go 1.27 or later.

```bash
go build -o bin/mcp-tiime ./cmd/mcp-tiime

# First login (hidden password prompt, MFA code requested when needed)
./bin/mcp-tiime login

# Check: lists the companies the account can access
./bin/mcp-tiime check
```

## Quick start

### Claude Code

```bash
claude mcp add tiime \
  -e TIIME_EMAIL=me@example.com \
  -e TIIME_PASSWORD=... \
  -- "$PWD/bin/mcp-tiime"
```

After rebuilding, restart the server with `/mcp` → **tiime** → **Reconnect**.

### Claude Desktop

```json
{
  "mcpServers": {
    "tiime": {
      "command": "/path/to/mcp-tiime/bin/mcp-tiime",
      "env": {
        "TIIME_EMAIL": "me@example.com",
        "TIIME_PASSWORD": "..."
      }
    }
  }
}
```

`TIIME_EMAIL` and `TIIME_PASSWORD` are optional once a session was opened with
`mcp-tiime login`. They let an agent log in again on its own when the session expires.

### Other clients

Cursor, VS Code, Windsurf, Zed, Codex CLI, Gemini CLI and any other stdio MCP client: see the
[MCP clients guide](docs/mcp-clients.md).

## Example requests

- "What are my last 10 bank transactions without a receipt?"
- "Match the October Qonto invoice with its bank debit."
- "Summarize my inflows and outflows over the last 30 days, by week."
- "How much VAT will I pay next month, and where does it come from?"
- "List the received e-invoices that are still unpaid."
- "Upload this receipt to the expenses to be reimbursed and add it to my November expense report."
- "Create a rule: every wording containing QONTO goes to the QONTO category."
- "Draft an invoice for EKINO: 20 days at €700 excluding VAT."

## Tools

Lists are paginated (`offset`, `limit` ≤ 100) and return `has_more`. When the account has a
single company, `company_id` can be omitted.

The **Tested** column shows what was checked against a real account: ✅ validated,
🟡 read-only validated, ⬜ never run for real (irreversible writes, or no real case available).

### Login

| Tool | Description | Tested |
|---|---|---|
| `start_login` | Starts the login. With MFA, Tiime sends a code (by email by default) and the tool returns `sent_at` | 🟡 |
| `complete_login` | Submits the received code. A wrong code returns a new challenge | ⬜ |

### Invoicing

| Tool | Description | Tested |
|---|---|---|
| `list_companies` | Accessible companies | ✅ |
| `list_clients`, `get_client` | Clients, billed amounts, outstanding balance, contacts | ✅ |
| `create_client`, `update_client` | Creates or updates a client | ⬜ |
| `list_invoices`, `get_invoice`, `download_invoice_pdf` | Invoices (most recent first), detail, PDF | ✅ |
| `list_vat_types`, `list_invoicing_units` | VAT codes with their rates, units (day, hour…) | ✅ |
| `create_invoice_draft`, `update_invoice_draft` | Invoice draft: totals and VAT computed; layout, seller details, bank details and terms copied from the latest invoice | ✅ creation |
| `delete_invoice_draft`, `duplicate_invoice` | Deletes a draft, duplicates an invoice | ✅ deletion |
| `finalize_invoice` | Numbers a draft (irreversible) | ⬜ |
| `send_invoice` | Emails the invoice, by default to the client billing emails with the Tiime template | ⬜ |
| `cancel_invoice`, `create_credit_note` | Cancels with a full credit note, or creates a partial credit note | ⬜ |
| `record_invoice_payment`, `list_invoice_payments`, `cancel_invoice_payment` | Payments received outside the bank (check, cash…) | 🟡 |
| `send_invoice_reminder`, `list_invoice_reminders` | Reminders for unpaid invoices | 🟡 |
| `list_quotes`, `get_quote` | Quotes | ✅ |
| `create_quote`, `update_quote`, `send_quote`, `duplicate_quote`, `delete_quote`, `create_invoice_from_quote` | Quote lifecycle, client acceptance or refusal, conversion to an invoice | ⬜ |
| `list_articles`, `save_article`, `delete_article` | Article catalogue | 🟡 |
| `list_einvoices` | E-invoices received through the e-invoicing platform (PDP): type (self-billed, credit note), status, remaining amount | ✅ |

### Banking and cash

| Tool | Description | Tested |
|---|---|---|
| `list_bank_accounts` | Bank accounts and balances | ✅ |
| `list_bank_transactions`, `get_bank_transaction` | Transactions: search, date filter, `matched` filter (without receipt) | ✅ |
| `cash_summary` | Inflows and outflows by day, week or month, with the largest movements | ✅ |
| `get_dashboard` | Inflows, outflows and balance of the month compared with the previous month, turnover, receivables, to-dos | ✅ |
| `annotate_bank_transactions` | Comment, accounting category or tags on one or several transactions | ✅ comment |
| `suggest_labels` | Categories suggested by Tiime for a transaction | ✅ |
| `list_transaction_documents` | Documents and invoices matched with a transaction | ✅ |
| `link_documents_to_transaction`, `unlink_documents_from_transaction` | Matches or unmatches receipts, keeping the other ones | ✅ matching |
| `link_invoices_to_transaction`, `unlink_invoices_from_transaction` | Matches or unmatches sales invoices (customer payments) | ⬜ |
| `get_forecast`, `set_forecast_enabled`, `save_forecast_row`, `delete_forecast_row` | Cash forecast | 🟡 |

### Categories, rules and tags

| Tool | Description | Tested |
|---|---|---|
| `list_labels`, `list_standard_labels` | Accounting categories of the company, standard catalogue | ✅ |
| `create_label`, `update_label` | Adopts a standard category or creates one, renames, archives | ⬜ |
| `list_label_rules`, `create_label_rule`, `delete_label_rule` | "Wording contains X → category Y" rules | ✅ |
| `list_tags`, `update_tag`, `delete_tag` | Tags: rename, archive, delete. A tag is created by applying it with `annotate_bank_transactions` | ✅ archiving |

### Documents

| Tool | Description | Tested |
|---|---|---|
| `list_document_categories`, `create_document_category`, `delete_document_category` | Categories of the documents space | ✅ |
| `list_documents` | Documents (most recent first) with date, wording, amount, VAT, number of matches | ✅ |
| `upload_document` | Uploads a local file (`receipt` category by default) with date, wording, amount, VAT | ✅ |
| `update_document`, `delete_document`, `download_document` | Updates, deletes or downloads a document | ✅ deletion |

### Expense reports and mileage allowances

| Tool | Description | Tested |
|---|---|---|
| `list_expense_reports`, `get_expense_report` | Expense reports (most recent first) with amount and VAT | ✅ |
| `create_expense_report` | Expense report from `advanced_expense` documents. Tiime rejects empty reports | 🟡 rejection checked |
| `update_expense_report`, `delete_expense_report` | Adds or removes expenses, submits a draft, deletes | ⬜ |
| `list_vehicles`, `list_travels`, `create_travel`, `estimate_travel_amount`, `create_mileage_report` | Vehicles, trips, official scale, mileage report | 🟡 |

### VAT and revenue book

| Tool | Description | Tested |
|---|---|---|
| `get_vat_estimate` | VAT of the month: collected, deductible, amount due, deadline, declaration status | ✅ |
| `get_vat_details` | Entries behind the collected or deductible VAT | ✅ |
| `list_revenue_book`, `export_revenue_book` | Revenue book, CSV or PDF export | 🟡 |

### Accounting firm

| Tool | Description | Tested |
|---|---|---|
| `list_accountant_requests` | Requested receipts and information, unread messages, pending expenses | 🟡 |
| `answer_document_request`, `declare_document_unavailable` | Answers a request, or declares a document unavailable with a reason | ⬜ |
| `list_threads`, `get_thread`, `post_thread_message`, `mark_thread_read` | Discussion threads with the firm | 🟡 |

## Accounting firms (expert mode)

With `TIIME_MODE=expert`, the server acts as an accounting firm on `expert.tiime.fr` instead of a
company: portfolio of client companies, task plannings, time tracking, VAT and annual accounts
plannings, accounting of each client (entries, trial balance, general ledger), receipt requests
and messaging with the clients. **Experimental: built from the web application code, not yet run
against a real firm account.** See the [expert mode guide](docs/expert-mode.md).

```bash
TIIME_MODE=expert ./bin/mcp-tiime login
claude mcp add tiime-expert -e TIIME_MODE=expert -- "$PWD/bin/mcp-tiime"
```

## Write safety

- Every tool carries MCP annotations: 38 are read-only (`readOnlyHint`) and 12 are marked
  destructive (`destructiveHint`), such as `finalize_invoice`, `cancel_invoice` and deletions.
  The MCP client can therefore ask for confirmation before running them.
- Writes that replace a whole set in Tiime (documents or invoices matched with a transaction)
  first read the current set and merge it: matching never removes another match.
- Inconsistent input is rejected before any call to Tiime (unknown quote status, negative
  amount, empty expense report, relative file path…).
- **Deliberately missing**: transfers from the Tiime business account, and sending or
  cancelling VAT returns. These actions are irreversible and deserve a dedicated safeguard
  before being handed to an agent.

## Authentication and MFA

The server keeps a refresh token in `TIIME_TOKEN_CACHE` (`0600` file) and renews its access
tokens, valid for 24 hours, on its own. A full login is only needed the first time or when
Tiime expires the session.

Two ways to log in when the account has two-factor authentication:

- **Through an agent**: the tools fail with a message asking to call `start_login`. Tiime
  emails a code. The agent reads it with a mail tool (for instance a Gmail MCP, looking for the
  message received after `sent_at`), then calls `complete_login`. The password stays in the
  server environment and never goes through the agent.
- **In a terminal**: `./bin/mcp-tiime login [email|sms|otp]` asks for the email and password
  (hidden input) when they are not in the environment, then for the code.

The login goes through the Auth0 Universal Login, like the web application, with PKCE.
Treat the cache as a secret: it grants access to the account.

## Configuration

All variables are optional.

| Variable | Default | Purpose |
|---|---|---|
| `TIIME_MODE` | `client` | `client` for a company (`apps.tiime.fr`), `expert` for an accounting firm (`expert.tiime.fr`, experimental, see [expert mode](docs/expert-mode.md)) |
| `TIIME_EMAIL`, `TIIME_PASSWORD` | — | Credentials, needed for `start_login` |
| `TIIME_COMPANY_ID` | detected | Default company, not needed when the account has a single one |
| `TIIME_PDF_DIR` | `~/Downloads` | Download directory (invoice PDFs, documents, exports) |
| `TIIME_TOKEN_CACHE` | `~/.config/mcp-tiime/token.json` (`token-expert.json` in expert mode) | Session cache |
| `TIIME_APP_VERSION` | version of the web application of the mode | `tiime-app-version` header, aligned with the web application |
| `TIIME_LOG_LEVEL` | `info` | Log level (JSON on stderr) |
| `TIIME_API_URL`, `TIIME_AUTH_URL`, `TIIME_CLIENT_ID` | web application values | Overrides in case Tiime changes them |

## Behaviors to know

Observed on a real account:

- **VAT on payment**: for services, the VAT of an invoice only shows up in
  `get_vat_estimate` once the payment is received.
- **Empty expense reports**: Tiime refuses to create an expense report without at least one
  expense.
- **E-invoices**: Tiime offers no server-side filter; `list_einvoices` scans the latest 500
  documents.
- **Tags**: there is no explicit creation. A tag is created when it is applied by name.
- **Custom categories**: creating them depends on an option enabled by the accounting firm.
  Standard categories can always be adopted.

The technical details of the API (routes, headers, formats and pitfalls) are in
[`docs/api-tiime.md`](docs/api-tiime.md).

## Development

```bash
go test -race ./...
golangci-lint run ./...     # v2.15 or later for Go 1.27
```

| Directory | Purpose |
|---|---|
| `cmd/mcp-tiime` | Entry point, `login` and `check` commands, `uber-go/fx` wiring |
| `internal/config` | Configuration with `viper` (`TIIME_*` variables) |
| `internal/tiime` | Tiime API HTTP client, Auth0 authentication (MFA, PKCE) |
| `internal/tools` | MCP tools (official `modelcontextprotocol/go-sdk`), one interface and one file per domain |

To add a tool: add the method to the client in `internal/tiime`, declare it in the domain
interface in `internal/tools`, register the tool in the domain `register…` function, then
update the expected list of `TestRegisterListsTools`. Project conventions and the full checklist are
described in [`CONTRIBUTING.md`](CONTRIBUTING.md).

## Contributing

Contributions are welcome: see [`CONTRIBUTING.md`](CONTRIBUTING.md). Report security issues
privately, as described in [`SECURITY.md`](SECURITY.md).

## License

[MIT](LICENSE). This project is not affiliated with Tiime.
