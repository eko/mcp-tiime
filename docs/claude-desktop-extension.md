# Claude Desktop extension

`mcp-tiime` ships as a Claude Desktop extension (`.mcpb` file): a one-click install, with no
terminal and no JSON to edit. Once installed, the Tiime tools are available in Claude Desktop
chats and in Cowork tasks.

> ⚠️ Community project, not affiliated with Tiime. It uses the internal API of the Tiime web
> application, which may change without notice.

## Contents

- [Requirements](#requirements)
- [Install](#install)
- [First login and two-factor authentication](#first-login-and-two-factor-authentication)
- [Use it in Cowork](#use-it-in-cowork)
- [Files: uploads and downloads](#files-uploads-and-downloads)
- [Update and uninstall](#update-and-uninstall)
- [Troubleshooting](#troubleshooting)

## Requirements

- Claude Desktop on macOS (Intel or Apple Silicon) or Windows.
- A Tiime account, with its email and password.

## Install

1. Download `mcp-tiime.mcpb` from the
   [latest release](https://github.com/eko/mcp-tiime/releases/latest/download/mcp-tiime.mcpb).
   A `mcp-tiime.mcpb.sha256` file sits next to it if you want to check the download.
2. Double-click the file. Claude Desktop opens the install dialog. You can also go to
   **Settings → Extensions → Advanced settings → Install Extension…** and pick the file.
3. Fill in the settings:

   | Setting | Purpose |
   |---|---|
   | Mode | `client` for a company account (default), `expert` for an accounting firm on Tiime Expert (experimental, see the [expert mode guide](expert-mode.md)) |
   | Tiime email | Email address of your Tiime account |
   | Tiime password | Stored by Claude Desktop in the system keychain, used to log in again when the session expires |
   | Company ID | Default company. Keep `0` when the account has a single company: it is detected |

4. Enable the extension. The Tiime tools show up in the tools menu of a new conversation.

To check the install, ask Claude: *"List my Tiime companies"*.

## First login and two-factor authentication

The server keeps a Tiime session in `~/.config/mcp-tiime/token.json` (on Windows,
`%USERPROFILE%\.config\mcp-tiime\token.json`) and renews it on its own. A full login is only
needed the first time, or when Tiime expires the session.

When a login is needed, the tools answer that Claude must call `start_login`. Then:

- **Without two-factor authentication**, the login completes right away.
- **With two-factor authentication**, Tiime sends a code (by email by default). If the Gmail
  connector is enabled, Claude can read the code itself and call `complete_login`. Otherwise,
  paste the code in the conversation.

The password never goes through the conversation: the server reads it from the extension
settings.

If you also use the `mcp-tiime` command line, `mcp-tiime login` writes the same session file, so
both share the session.

## Use it in Cowork

Cowork uses the extensions installed in Claude Desktop: there is nothing else to set up. Some
tasks that work well:

- *"Find my bank transactions from October without a receipt, and match the PDFs from my
  Receipts folder to them."*
- *"Create my November expense report from the receipts I uploaded this month."*
- *"Draft an invoice for EKINO: 20 days at €700 excluding VAT, then show it to me before
  finalizing."*
- *"How much VAT will I pay next month, and which invoices does it come from?"*
- *"Which documents is my accounting firm still waiting for?"*

Tools that change data irreversibly (finalizing or cancelling an invoice, deletions…) are
marked as destructive, so Claude asks for your approval before running them. Transfers from
the Tiime business account and VAT returns are deliberately not available.

## Files: uploads and downloads

The server runs on your computer, not in the cloud:

- `upload_document` reads a file from your computer and expects its **absolute path**, for
  instance `/Users/me/Documents/Receipts/amazon-october.pdf`. If Claude gives a path that does
  not exist, give it the full path of the file on your computer.
- Downloads (invoice PDFs, documents, revenue book exports) go to your `Downloads` folder.

## Update and uninstall

- **Update**: download the new `mcp-tiime.mcpb` from the releases and install it again. Your
  settings are kept.
- **Uninstall**: **Settings → Extensions**, then remove **Tiime (community)**. To also drop the
  Tiime session, delete `~/.config/mcp-tiime/token.json`.

## Troubleshooting

| Symptom | What to do |
|---|---|
| The extension does not start | Check the server logs: `~/Library/Logs/Claude/` on macOS, `%APPDATA%\Claude\logs\` on Windows (files named after the extension) |
| Every tool asks for `start_login` | The session expired or was revoked: let Claude call `start_login` (or run `mcp-tiime login` if you have the command line) |
| `start_login` fails before any code is sent | Check the email and password in **Settings → Extensions → Tiime (community)** |
| Several companies, Claude asks which one | Set the **Company ID** setting, or say which company to use. `list_companies` shows the IDs |
| A tool that worked suddenly fails with an HTTP error | Tiime may have changed its internal API: check for a newer release or open an [issue](https://github.com/eko/mcp-tiime/issues) |
