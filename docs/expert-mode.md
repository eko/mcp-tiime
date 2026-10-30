# Expert mode (accounting firms)

> 🧪 **Experimental.** The expert mode was built from the code of the `expert.tiime.fr` web
> application, without access to an accounting firm account. **No tool has been run against a
> real account yet.** Read the [beta test plan](#beta-test-plan) before using it on client files.

`mcp-tiime` normally acts as a company using `apps.tiime.fr`. With `TIIME_MODE=expert`, it acts
as an accounting firm using **Tiime Expert** (`expert.tiime.fr`): the agent works across the
firm's portfolio of client companies.

## Contents

- [Set up](#set-up)
- [Tools](#tools)
- [Deliberately missing](#deliberately-missing)
- [Beta test plan](#beta-test-plan)
- [How it differs from the client mode](#how-it-differs-from-the-client-mode)

## Set up

The expert mode uses the same binary, with `TIIME_MODE=expert`:

```bash
# First login with the Tiime Expert account (MFA included)
TIIME_MODE=expert ./bin/mcp-tiime login

# Check: lists the first client companies of the portfolio
TIIME_MODE=expert ./bin/mcp-tiime check
```

Then add `TIIME_MODE=expert` to the environment of your MCP client, for instance with Claude
Code:

```bash
claude mcp add tiime-expert \
  -e TIIME_MODE=expert \
  -e TIIME_EMAIL=me@firm.example \
  -e TIIME_PASSWORD=... \
  -- "$PWD/bin/mcp-tiime"
```

In the Claude Desktop extension, set **Mode** to `expert`. The other clients take the same
variable (see [MCP clients](mcp-clients.md)).

- The expert session is cached in its own file, `~/.config/mcp-tiime/token-expert.json`, so a
  client session and an expert session can coexist.
- `TIIME_COMPANY_ID` optionally sets the client company used by tools called without
  `company_id`. Otherwise, the agent finds the company with `list_portfolio_companies`.
- The Tiime Expert OAuth client does not accept direct password logins: when the session
  expires, tools answer that `start_login` is needed, and the agent logs in again (MFA code
  included, see [Authentication and MFA](../README.md#authentication-and-mfa)).

## Tools

All tools are ⬜ never run for real. 43 tools in total, including `start_login` and
`complete_login`.

### Portfolio, tasks and time tracking

| Tool | Description |
|---|---|
| `get_expert_me` | Firm user logged in, with the ID used by the user filters |
| `list_portfolio_companies`, `get_portfolio_company` | Client companies: search, mission status, tax regime, closing month, contributors, business units |
| `list_tasks` | Task plannings (missions with a deadline) by status, deadline, overdue, invoicing, people, or all the tasks of a company, with optional totals |
| `create_task` | Task planning, optionally recurring every N months. Defaults: you as responsible, the default invoicing type |
| `update_task` | Status, label, deadline, invoicing, people. Reads the task and sends it back whole, as the web application does. Can apply to the next occurrences |
| `delete_task` | Deletes a task planning, optionally with its next occurrences (destructive) |
| `set_task_billing_check` | Billing check mark of a task |
| `get_time_tracking_options` | Categories, task types, invoicing types, duration types |
| `list_time_records`, `get_time_summary` | Time spent on a company over a period, records of a user for a day, recorded and validated time per day |
| `save_time_record`, `delete_time_record`, `validate_time` | Records time (duration in minutes; the category is taken from the task type), deletes a record, validates or reopens days |

### Accounting of a client company

| Tool | Description |
|---|---|
| `list_accounting_periods` | Fiscal years, open or closed |
| `list_movements` | Entry lines by fiscal year or dates, account (general ledger), journal, amount, lettering, validation. Amounts are signed: credit positive, debit negative |
| `get_accounting_entry` | Every line of an entry |
| `get_account_summary` | Balances of an account per fiscal year, and debit and credit totals over a period |
| `get_trial_balance` | Trial balance of a fiscal year, by account roots or balance side, with the accounting result |
| `list_chart_of_accounts`, `list_journals`, `list_company_labels` | Accounts, journals, categories of the client |
| `validate_movements` | Validates (visa) entry lines, or removes the validation |
| `list_client_bank_transactions` | Bank transactions of the client with their pending receipt requests |
| `request_receipts`, `cancel_receipt_requests` | Asks the client for the receipts of transactions (they appear in the client's to-do list in the Tiime app), or withdraws the requests |

### Follow-up of the portfolio

| Tool | Description |
|---|---|
| `list_vat_planning` | VAT declarations to handle across the portfolio: deadline, status, consistency checks, overdue |
| `list_company_vat_declarations`, `get_vat_declaration` | VAT declarations of a company, with the form and its consistency checks |
| `download_vat_declaration` | Declaration, acknowledgment receipt, proofs of filing… to the download directory |
| `list_bilan_planning`, `get_bilan` | Annual accounts to produce across the portfolio, and their detail |
| `list_client_indicators` | What each client still has to do: items to complete, unanswered messages, document requests, bank connections, suspense account entries |
| `refresh_client_indicators`, `list_items_to_relaunch` | Recomputes the indicators of a company, lists the transactions waiting for a receipt |
| `list_threads`, `create_thread`, `post_thread_message`, `update_thread` | Messaging with the client, about a transaction, a task, a VAT declaration or a fiscal year. Messages are visible to the client |
| `list_notifications`, `mark_notifications_read` | Notifications of the firm user |

### Example requests

- "Which VAT declarations are due before the 24th and still in draft?"
- "List my overdue tasks, grouped by company."
- "For ACME, find the October bank transactions without a receipt and ask the client for them."
- "Which clients have items to complete or unanswered messages?"
- "Show the trial balance of ACME for 2026, accounts 4 with a debit balance."
- "Record 1h30 of bookkeeping on ACME today."

## Deliberately missing

These actions exist in Tiime Expert but are not exposed, because a mistake cannot be undone or
reaches the tax administration or the client:

- sending, changing the status of, or resetting VAT declarations, tax returns (liasses) and
  annual accounts;
- creating, changing or deleting accounting entries, imports, closing or reopening fiscal
  years;
- sending email reminders to clients (`company_accounting_indicators/send`): the request does
  not say which company it is for, so it could not be checked safely;
- the firm settings (business units, users, accounting plans, cover pages…).

## Beta test plan

Start with read-only tools, then try the writes on a **test or demo company** of the firm.
Report each result in an issue (tool, arguments, result or error, without client data).

1. **Login**: `TIIME_MODE=expert mcp-tiime login`, then `mcp-tiime check`. The login goes
   through `/co/authenticate` with the `expert.tiime.fr` origin, which Tiime accepts, but the
   end of the flow was never run.
2. **Portfolio**: `get_expert_me`, `list_portfolio_companies` (with and without `status`),
   `get_portfolio_company`.
3. **Plannings**: `list_tasks`, `list_vat_planning`, `list_bilan_planning`,
   `list_client_indicators`.
4. **Accounting**: `list_accounting_periods`, `list_movements` with `accounting_period_id`,
   `get_trial_balance`, `get_account_summary`, `list_client_bank_transactions`.
5. **Writes, on a test company**: `post_thread_message`, `update_thread` (read),
   `request_receipts` then `cancel_receipt_requests`, `create_task` then `update_task` then
   `delete_task`, `save_time_record` then `delete_time_record`, `validate_movements` with
   `validated=true` then `false`.

Points to check in particular, as the web application code leaves them open:

| Point | Tool |
|---|---|
| A new task accepts `{id}` alone for the company and the people (the web application sends full objects) | `create_task` |
| A new thread accepts an empty list of interlocutors | `create_thread` |
| `closing_month` counts months from 1 to 12 | `list_portfolio_companies` |
| The `X-Total-Not-Configured-Company` header and the pagination of the VAT planning | `list_vat_planning` |
| The direct bank transactions route returns the receipt requests | `list_client_bank_transactions` |
| A refresh token is issued, so the session lasts beyond 24 hours | any tool, the next day |

## How it differs from the client mode

| | Client mode | Expert mode |
|---|---|---|
| Web application | `apps.tiime.fr` | `expert.tiime.fr` |
| OAuth client | `iEbsbe3o66gcTBfGRa012kj1Rb6vjAND` | `ExMoxAnc9Y8BstgApTW7VpiUh63gmPCd` |
| `tiime-app` header | `tiime` | `tiime-pro` |
| Password grant | accepted | refused: always the login pages |
| Routes | `/companies`, `/accounts/companies`, `/invoice/companies` | `/expert/...` |
| Session cache | `token.json` | `token-expert.json` |

The routes are documented in [`api-tiime.md`](api-tiime.md#expert-application).
