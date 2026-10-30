# Tiime internal API

Reverse-engineering notes on the API used by the `apps.tiime.fr` web application, extracted
from its JavaScript code (version `4.40.6`) and checked against a real account. They help fix
the client when Tiime changes something.

To find a route in a new version of the application: download `index.html` and the
`releases/<hash>/main-*.js` and `chunk-*.js` files it references, then search for the route
(for instance `expense_reports`). Services call `this.http.get(j('api/v1/...'))`, and models
expose `fromJson`/`toJson`, which give the field names.

## Basics

| Item | Value |
|---|---|
| API | `https://chronos-api.tiime-apps.com/v1` (the application writes `api/v1/...` and drops `api/`) |
| Auth0 | `https://auth0.tiime.fr`, realm `Chronos-prod-db`, audience `https://chronos/` |
| OAuth client | `iEbsbe3o66gcTBfGRa012kj1Rb6vjAND` (public, the web application one) |
| Headers sent on every call | `tiime-app: tiime`, `tiime-app-platform: web`, `tiime-app-version: <version>` |
| Authentication | `Authorization: Bearer <access token>`, valid for 24 hours |

Three prefixes coexist for the same company:

- `/companies/{id}/…`: legacy (invoices, clients, categorization rules, collected payments);
- `/accounts/companies/{id}/…`: accounting (banking, documents, expense reports, VAT, categories, tags);
- `/invoice/companies/{id}/…`: invoicing (invoices, quotes, clients, articles, reminders).

## Authentication

1. `POST /oauth/token` with `grant_type=http://auth0.com/oauth/grant-type/password-realm`.
   Without MFA, it returns the tokens directly. With MFA, it returns
   `403 {"error":"mfa_required"}`.
2. With MFA, the application goes through the Universal Login:
   - `POST /co/authenticate` (with `Origin: https://apps.tiime.fr`) returns a `login_ticket`;
   - `GET /authorize?response_type=code&login_ticket=…&code_challenge=…` redirects to the
     `/u/mfa-…` pages;
   - `/u/mfa-login-options` page: `action=<method>::<index>` buttons (`email::1`, `sms::0`…);
   - `/u/mfa-<method>-challenge` page: `state` + `code` + `action=default` form; the
     `pick-authenticator` button goes back to the method choice;
   - the last redirection targets `https://apps.tiime.fr/auth-callback?code=…`.
3. `POST /oauth/token` with `grant_type=authorization_code` and the PKCE `code_verifier`.
   With the `offline_access` scope, the response contains a refresh token.

Universal Login forms require `Accept: text/html` (otherwise `406`) and the cookies set by
`/co/authenticate`.

## Pagination and sorting

- Request: `Range: items=<start>-<end>`, **inclusive** bounds (`items=0-9` returns 10 items).
- Response: `Content-Range: items 0-9/120`, or `items 0-9/*` when the total is unknown (more
  items remain). An empty page may answer `204` without a body.
- Categories (`labels`) are capped at 100 per page.
- Sorting: `sorts=<field>:<asc|desc>`, several separated by commas. It works on `accounts`
  routes (`date:desc`, `created_at:desc`). **On invoices, adding `sorts` breaks the order**:
  without the parameter, the most recent comes first.
- Multiple values in a filter are joined with `|` (`status=opened|closed`).
- Date and amount filters: comma separated conditions, for instance
  `date=>=2026-10-01,<=2026-10-31` or `emission_date=>=2026-01-01`.

Some lists wrap their items: `{"transactions": […], "metadata": {…}}` with the
`bank_transactions.v2` Accept, `{"expense_reports": […]}`, `{"travels": […]}`,
`{"vehicles": […]}`, `{"results": […]}` for VAT details.

## Accept headers

| Accept | Usage |
|---|---|
| `application/vnd.tiime.bank_transactions.v2+json` | Transactions with `count_documents` and `count_invoices` at the root, and `metadata.total_amount` |
| `application/vnd.tiime.bank_transactions.without_documents+json` | Light transactions: document counts are **not** reliable |
| `application/vnd.tiime.docs.imputation+json` | Transaction detail with its imputations |
| `application/vnd.tiime.bank_transactions.label_suggestions.v2+json` | Category suggestions |
| `application/vnd.tiime.doc.pdp+json` | Documents, with the e-invoice fields (`is_pdp`, `pdp_*`) |
| `application/vnd.tiime.labels.v2+json` | Categories, with `rules_count` when `with_rules_count=true` |
| `application/vnd.tiime.todo.module_type_with_mileage_expense+json` | Accounting firm requests |
| `application/vnd.tiime.tile.todo.v3+json` | Dashboard tiles |
| `application/vnd.tiime.vehicles.v2+json` | Vehicles |
| `application/vnd.tiime.accountant_request.document_unavailable.v2+json` | Declaring a document unavailable |

## Checked pitfalls

| Topic | Behavior |
|---|---|
| Document counts | They sit at the root of the transaction. Imputation counts are always 0 |
| Matching | `PUT …/document_matchings` and `PUT …/invoice_matchings` **replace** the whole set: read `GET …/matchings` first and merge |
| Empty comment | `""` is silently ignored; `null` clears the comment |
| Document dates | `date` metadata of type `datetime`, value `"YYYY-MM-DD 00:00:00"` |
| Document VAT | Either `{"value": x}` (type `amount`), or a list per rate with a `total` entry (type `amounts`) whose value may be `null` |
| Tags and categories | `PATCH` calls send the application model as is: **camelCase** keys (`archivedAt`, `isAutomatic`, `registryType`) |
| Tag creation | No route: a tag is created when it is applied by name |
| Expense reports | `advanced_expenses` must contain at least one ID, otherwise `400` |
| Expense report update | `PATCH` with the full body; the expense list replaces the previous one |
| Expense report deletion | No dedicated route: it is a document, deleted with `DELETE /documents/{id}` |
| Forecast | `GET /forecast_balance` answers `204` when not activated, and rows answer `404` |
| Estimated VAT | `month` and `year` are required, otherwise `400` |

## Routes by domain

`{c}` is the company ID. ✅: checked with a write on a real account.

### Banking

| Method and route | Usage |
|---|---|
| `GET /accounts/companies/{c}/bank_accounts` | Bank accounts |
| `GET /accounts/companies/{c}/bank_transactions` | Transactions. Filters `q`, `bank_account`, `date`, `matched=true\|false`, `amount_type`, `to_do_list_items_type` |
| `GET /accounts/companies/{c}/bank_transactions/{id}` | Detail |
| `PATCH /accounts/companies/{c}/bank_transactions` | Bulk: `{bank_transaction_ids, bank_transaction: {comment} \| {imputations: [{label: {id}}]} \| {tags: [{name}]}}` ✅ |
| `GET /accounts/companies/{c}/bank_transactions/{id}/matchings` | Matched documents (`type` = category) and invoices (`type: invoice`) |
| `PUT /accounts/companies/{c}/bank_transactions/{id}/document_matchings` | `{documents: [{id}]}` ✅ |
| `PUT /accounts/companies/{c}/bank_transactions/{id}/invoice_matchings` | `[{id}]` |
| `GET /accounts/companies/{c}/bank_transactions/{id}/label_suggestions` | Suggested categories |

### Categories, rules and tags

| Method and route | Usage |
|---|---|
| `GET /accounts/companies/{c}/labels` | Categories (`q`, `with_rules_count`) |
| `GET /accounts/companies/{c}/standard_labels` | Standard catalogue |
| `POST /accounts/companies/{c}/labels` | Creates a category or adopts a standard one (camelCase) |
| `PATCH /accounts/companies/{c}/labels/{id}` | `{id, label}` to rename, `{id, disabled}` to archive |
| `GET / POST /companies/{c}/label_rules`, `DELETE /companies/{c}/label_rules/{id}` | Rules `{label: {id}, wording}` ✅ |
| `GET /accounts/companies/{c}/tags?expand=tag_detail` | Tags |
| `PATCH /accounts/companies/{c}/tags/{id}` | Full tag in camelCase, `archivedAt` to archive ✅ |
| `DELETE /accounts/companies/{c}/tags/{id}` | Deletion |

### Documents

| Method and route | Usage |
|---|---|
| `GET /accounts/companies/{c}/document_categories` | Categories (`identifier`: `receipt`, `advanced_expense`, `external_invoice`…) |
| `POST /accounts/companies/{c}/document_categories`, `DELETE …/{id}` | `{name}` ✅ |
| `GET /accounts/companies/{c}/documents` | Documents (`document_categories`, `q`, `sorts=created_at:desc`) |
| `POST /accounts/companies/{c}/document_categories/{cat}/documents` | Multipart upload: `file` and `source=web` fields ✅ |
| `PATCH /accounts/companies/{c}/documents/{id}` | `{metadata: […], comment}` ✅ |
| `GET /accounts/companies/{c}/documents/{id}/file` | File |
| `DELETE /accounts/companies/{c}/documents/{id}` | Deletion ✅ |

### Invoicing

| Method and route | Usage |
|---|---|
| `GET /invoice/invoicing_config/vat_types` | VAT codes and dated rates (`rate` as a fraction) |
| `GET /invoice/companies/{c}/invoicing_config` | Defaults (`number_template`, `due_date_mode`) |
| `GET /invoice/companies/{c}/countries/FR/invoicing_translations` | Default email subjects and bodies |
| `POST /invoice/companies/{c}/invoices` | Creation: full contract, `status: draft` ✅ |
| `PATCH /invoice/companies/{c}/invoices/{id}` | Update; `status: saved` + `number_template` to number it |
| `DELETE /invoice/companies/{c}/invoices/{id}/draft` | Draft deletion ✅ |
| `POST …/invoices/{id}/send?email=true&pdp=false` | Sending: `{from, to, cc, cci, subject, body, reply_to, filename, extra_attachments}` |
| `POST …/invoices/{id}/duplicate`, `…/credit_note`, `…/corrective_invoice` | Duplication, credit note (`{emission_date, cancel_invoice: true}` to cancel) |
| `POST /companies/{c}/collected_payments` | Manual payment `{invoice_id, date, amount, operation_type, source: manual, label}` |
| `POST /invoice/companies/{c}/dunnings` | Reminder `{invoice: {id}, mail: {…}}` |
| `… /invoice/companies/{c}/quotations` | Quotes: same contract as invoices, plus `validity_period`, `acceptance_conditions`, `signature_enabled` |
| `… /invoice/companies/{c}/articles` | Catalogue `{code, description, unit_amount, vat_type: {code}, invoicing_unit, invoicing_category_type}` |
| `… /invoice/companies/{c}/clients` | Clients `{professional, name, siren_or_siret, intracom_vat_number, email, invoicing_use_email, …}` |

An invoice contract contains a copy of the client (`client_name`, `client_address`…), the
seller (`seller`), the lines `{description, quantity, unit_amount, line_amount, vat_type: {code},
invoicing_unit, invoicing_category_type, sequence}` and totals computed by the client
(`total_excluding_taxes`, `total_including_taxes`, `totals_per_vat_type`). The server computes
the due date from `due_date_mode`.

### Expense reports

| Method and route | Usage |
|---|---|
| `GET /accounts/companies/{c}/expense_reports?expand=preview_available&sorts=date:desc` | List |
| `POST /accounts/companies/{c}/expense_reports?expand=preview_available` | `{name, date, owner, expense_type: advanced_expense\|travel, advanced_expenses, travels, lifecycle_status: draft\|saved, …}` |
| `PATCH /accounts/companies/{c}/expense_reports/{id}?expand=preview_available` | Full body |
| `GET / POST /accounts/companies/{c}/travels`, `GET …/travels/estimated_amount` | Trips and official scale |
| `GET /accounts/companies/{c}/vehicles` | Vehicles |

### VAT, dashboard, forecast

| Method and route | Usage |
|---|---|
| `GET /accounts/companies/{c}/estimated_vat?month=&year=` | VAT of the month |
| `GET /accounts/companies/{c}/estimated_vat/{collected\|deductible}?month=&year=` | Underlying entries |
| `GET /accounts/companies/{c}/tiles?keys=…` | Tiles (`in_out_evolution`, `to_do_counts`, `month_to_declare`…) |
| `GET / PATCH /accounts/companies/{c}/forecast_balance` | Activation `{activated_at}` |
| `… /accounts/companies/{c}/forecast_balance_rows` | Rows `{name, flow_type, interval_values: [{date, value}]}` |
| `GET /invoice/companies/{c}/registries`, `…/registries/{csv\|pdf}` | Revenue book |

### Accounting firm

| Method and route | Usage |
|---|---|
| `GET /accounts/companies/{c}/to_do_list_items` | Requests and items to handle |
| `PATCH /accounts/companies/{c}/accountant_detail_request/document_requests/{id}` | Answer `{document_ids}` |
| `PATCH …/bank_transactions/{tx}/accountant_detail_requests/{id}/document_declared_unavailable` | `{document_declared_unavailable_at, document_declared_unavailable_reason, message_content}` |
| `GET /accounts/companies/{c}/threads`, `GET …/threads/{id}` | Discussion threads |
| `POST …/threads/{id}/messages`, `POST …/threads/{id}/read` | Message `{content}`, mark as read (body `true`) |

### Deliberately not implemented

| Route | Reason |
|---|---|
| `/wallet/…` (transfers, beneficiaries, strong authentication) | Money movements |
| `POST …/estimated_vat/{id}/send`, `/cancel`, `/validate` | Tax returns |
| `/accounts/companies/{c}/social/…` | Payroll, not accessible with the tested plan (`403`) |

## Expert application

`expert.tiime.fr` (Tiime Expert, for accounting firms) uses the same API and Auth0 tenant, with
its own client. Extracted from its JavaScript code (version `5.66.0`); **never checked against
a real account**.

| Item | Value |
|---|---|
| OAuth client | `ExMoxAnc9Y8BstgApTW7VpiUh63gmPCd` (public), Auth0 SPA SDK |
| Headers | `tiime-app: tiime-pro`, `tiime-app-version: 5.66.0:<build hash>` |
| Password-realm grant | **refused** (`unauthorized_client`): every login goes through the pages |
| `/co/authenticate` | accepted with `Origin: https://expert.tiime.fr` |
| Redirect URI | `https://expert.tiime.fr/auth-callback` (registered; any other gives `403`) |

Services are Angular classes with a `resourceUrl`: search the bundles for
``resourceUrl=`api/v1/expert/…` `` to find a route and its methods. `${P}` in a URL stands for
the company ID. The base service maps `getById`/`getAll` to `GET`, `getPaginated` to `GET` with
`Range`, `create` to `POST`, `update` to `PATCH` and `delete` to `DELETE`. Lists across the
portfolio take common filters: `contributor_ids` (`|`), `business_units` (**`,`**),
`mission_ended`, `mission_works_stopped`, `in_creation_process`, `initialization_in_progress`.

### Portfolio, tasks, time tracking

| Method and route | Usage |
|---|---|
| `GET /expert/users/me` | Firm user |
| `GET /expert/companies` | Client companies (`q`, `sorts=name:…`, `tax_regime_code`, `closing_month`, status flags) |
| `GET /expert/companies/{c}` | Company (`expand=legal_company,pdp_config,thread`) |
| `GET /expert/users/me/tasks` | Task plannings (`status`, `date`, `out_of_deadline`, `invoicing_status`, `responsable_ids`, `task_ids`…); `?company={c}` without `Range` for one company |
| `GET /expert/users/me/tasks/totals` | `{amount, discount, amount_after_discount}` |
| `POST /expert/tasks` | Task `{wording, deadline_date, company, time_tracking_task:{id}, time_tracking_invoicing_type:{id}, responsable, supervisor, invoicing_status, amount, discount, task_scheduler}`; recurrence `{interval, day (null = end of month), first_deadline_date, last_deadline_date \| occurence}` |
| `PATCH / DELETE /expert/companies/{c}/tasks/{id}[?apply_nexts=<deadline>]` | Whole task body; `apply_nexts` also changes the next occurrences |
| `POST /expert/companies/{c}/tasks/{id}/billing_point`, `/billing_unpoint` | Body `{}` |
| `GET /expert/time_tracking/{categories,tasks,invoicing_types,duration_types}` | Reference data; tasks take `time_tracking`, `prestation` or `additional_mission=true` |
| `GET /expert/time_tracking/records?user_id=&date=` | Records of a user for a day |
| `PUT /expert/time_tracking/records` | Creates a record, or updates it when the body has an `id`; `duration` in minutes |
| `DELETE /expert/time_tracking/records/{id}` | |
| `POST /expert/time_tracking/records/validate` | `{start_date, end_date, user_ids, validated}` |
| `GET /expert/time_tracking/records/days?owner_ids=&start_date=&end_date=` | `{date, validated, duration}` per day |
| `GET /expert/companies/{c}/time_tracking/records`, `…/totals` | Time on a company (`date`, `user_id`, `gi_task_id`…) |

### Accounting

| Method and route | Usage |
|---|---|
| `GET /expert/companies/{c}/accounting_periods` | Fiscal years `{id, closed, start_date, end_date}` |
| `GET /expert/companies/{c}/movements` | Entry lines (`expand=lettering,accounting_entry_counterpart,vat_amount,has_attachment`, `date`, `pcg_entry`, `account_number_prefixes` (`\|`), `journal_code` (`,`), `amount`, `lettered`, `validated`…). `amount` is **positive for a credit, negative for a debit**. Filter a fiscal year with `date`, not `accounting_period` |
| `GET /expert/companies/{c}/movements/cumuls?pcg_entry=&date=` | `{count, total, total_debit, total_credit}` |
| `GET /expert/companies/{c}/accounting_entries/{id}/movements` | Lines of an entry |
| `POST /expert/companies/{c}/movements/validate`, `/unvalidate` | `{movements: [{id}]}` |
| `GET /expert/companies/{c}/accounting_periods/{p}/balances` | Trial balance; balance = credit − debit; `amount=<0` for debit balances |
| `GET /expert/companies/{c}/accounting_periods/{p}/results?items=accounting_result` | Accounting result |
| `GET /expert/companies/{c}/pcg_entries`, `…/pcg_entries/{id}/balances` | Chart of accounts, balances of an account per fiscal year |
| `GET /expert/companies/{c}/journals[?type=]` | Journals |
| `GET /expert/companies/{c}/labels` | Categories of the client |
| `GET /expert/companies/{c}/bank_transactions` | Transactions (`Accept: …bank_transactions.v2+json`, `expand=imputations`, `start_date=>=…`) |
| `POST / DELETE /expert/companies/{c}/bank_transactions/accountant_detail_requests` | Receipt requests: `{type: "receipt", bank_transactions: [{id}]}`; the DELETE has a body `{type, accountant_detail_requests: [{id}]}` |

### Follow-up

| Method and route | Usage |
|---|---|
| `GET /expert/users/me/vat_declarations` | VAT planning; header `X-Total-Not-Configured-Company: true`, `expand=accounting_config`, `vat_declaration_status`, `vat_declaration_deadline_start_date`/`_end_date` (plain dates), `vat_declaration_consistency_check_status`… |
| `GET /expert/companies/{c}/vat_declarations[/{id}]` | VAT declarations of a company |
| `GET /expert/companies/{c}/vat_declarations/{id}/consistency_check` | `Accept: application/vnd.tiime.consistency_checks.v4+json` |
| `POST /expert/companies/{c}/vat_declarations/{id}/pdf` | `{type}` → file (`vat_declaration`, `acknowledgment_receipt`, `fiscal_declaration`, `proofs_of_filing`, `vat_deposit`) |
| `GET /expert/users/me/bilans` | Annual accounts planning (`bilan_status`, `accounting_period_end_date`…) |
| `GET /expert/companies/{c}/bilans/{id}?expand=consistency_checks,metadatas` | Annual accounts |
| `GET /expert/company_accounting_indicators` | Indicators per client (`expand=` the indicator codes); `id` is the indicator row, not the company |
| `POST /expert/company_accounting_indicators/update` | `{company_ids: [c]}` recomputes |
| `GET /expert/company_accounting_indicators/{id}/items_to_relaunch/bank_transactions` | Transactions waiting for a receipt |
| `GET / POST /expert/companies/{c}/threads` | Threads (`status`, `category`, `types` + the entity ID under the name of its type); creation `{entity: {type, <type>: {id}}, interlocutors, messages: [{content}], category, tags}` |
| `POST /expert/companies/{c}/threads/{id}/messages` | `{content}` |
| `POST /expert/companies/{c}/threads/{id}/{read,close,unclose,archive,unarchive}` | No body |
| `GET /expert/users/me/notifications[/count]`, `PATCH …/{id}`, `POST …/mark_all_as_read` | Notifications |

### Deliberately not implemented (expert)

| Route | Reason |
|---|---|
| `PATCH / PUT …/vat_declarations/{id}`, `POST …/fiscal_declarations/{id}/send`, `POST …/bilans/{id}/send` | Tax returns and annual accounts |
| `POST …/movements`, `POST …/accounting_entries/{id}/movements`, `…/accounting_entry_imports` | Accounting entries |
| `POST /expert/company_accounting_indicators/send` | Email to the client; the body names no company |
