# MCP clients

`mcp-tiime` is a standard MCP server over stdio: any MCP client that can launch a local command
can use it. This guide gives the setup for the most common ones.

For Claude Desktop and Cowork, the one-click extension is the easiest way: see the
[Claude Desktop extension guide](claude-desktop-extension.md).

## Contents

- [Before you start](#before-you-start)
- [Claude Code](#claude-code)
- [Claude Desktop (manual setup)](#claude-desktop-manual-setup)
- [Cursor](#cursor)
- [VS Code](#vs-code)
- [Windsurf](#windsurf)
- [Zed](#zed)
- [Codex CLI](#codex-cli)
- [Gemini CLI](#gemini-cli)
- [Other clients](#other-clients)
- [Running through Docker](#running-through-docker)

## Before you start

### 1. Get the binary

Download the archive for your system from the
[latest release](https://github.com/eko/mcp-tiime/releases/latest), or build it from source
(see the [README](../README.md#from-source)). Put the binary somewhere stable and note its
**absolute path**. The examples below use `/usr/local/bin/mcp-tiime`; on Windows, use the full
path to `mcp-tiime.exe` with escaped backslashes in JSON (`C:\\Tools\\mcp-tiime.exe`).

### 2. Choose how the server logs in

| Approach | Setup | When the session expires |
|---|---|---|
| **Session only** (recommended) | Run `mcp-tiime login` once in a terminal, and give the client no credentials | Run `mcp-tiime login` again |
| **Credentials in the client** | Set `TIIME_EMAIL` and `TIIME_PASSWORD` in the client config | The agent logs in again with `start_login` / `complete_login`, MFA code included |

Both approaches share the session cache (`~/.config/mcp-tiime/token.json`), so the CLI and every
client use the same session. Credentials in a config file are stored in plain text: prefer a
client that can prompt for secrets (VS Code inputs) or read them from your environment.

Accounting firms on Tiime Expert add `TIIME_MODE=expert` to the environment, next to the
credentials: see the [expert mode guide](expert-mode.md).

The examples below show the credentials variables. Drop the `env` block for the session only
approach. Every [configuration variable](../README.md#configuration) can be set the same way,
for instance `TIIME_COMPANY_ID` when the account has several companies.

## Claude Code

```bash
claude mcp add tiime \
  -e TIIME_EMAIL=me@example.com \
  -e TIIME_PASSWORD=... \
  -- /usr/local/bin/mcp-tiime
```

Add `--scope user` to make it available in every project. After upgrading the binary, restart
the server with `/mcp` → **tiime** → **Reconnect**.

## Claude Desktop (manual setup)

Without the extension, edit `claude_desktop_config.json` (**Settings → Developer → Edit
Config**), then restart Claude Desktop:

```json
{
  "mcpServers": {
    "tiime": {
      "command": "/usr/local/bin/mcp-tiime",
      "env": {
        "TIIME_EMAIL": "me@example.com",
        "TIIME_PASSWORD": "..."
      }
    }
  }
}
```

## Cursor

Edit `~/.cursor/mcp.json` for every project, or `.cursor/mcp.json` in a project:

```json
{
  "mcpServers": {
    "tiime": {
      "command": "/usr/local/bin/mcp-tiime",
      "env": {
        "TIIME_EMAIL": "me@example.com",
        "TIIME_PASSWORD": "..."
      }
    }
  }
}
```

Then check that **tiime** is enabled in **Cursor Settings → MCP**.

## VS Code

Run **MCP: Open User Configuration** from the command palette (or create `.vscode/mcp.json` in
a workspace). VS Code asks for the password on the first start and stores it securely, so it
never sits in the file:

```json
{
  "inputs": [
    {
      "type": "promptString",
      "id": "tiime-password",
      "description": "Tiime password",
      "password": true
    }
  ],
  "servers": {
    "tiime": {
      "type": "stdio",
      "command": "/usr/local/bin/mcp-tiime",
      "env": {
        "TIIME_EMAIL": "me@example.com",
        "TIIME_PASSWORD": "${input:tiime-password}"
      }
    }
  }
}
```

The tools are available in Copilot Chat agent mode.

## Windsurf

Edit `~/.codeium/windsurf/mcp_config.json` (`%USERPROFILE%\.codeium\windsurf\mcp_config.json`
on Windows):

```json
{
  "mcpServers": {
    "tiime": {
      "command": "/usr/local/bin/mcp-tiime",
      "env": {
        "TIIME_EMAIL": "me@example.com",
        "TIIME_PASSWORD": "..."
      }
    }
  }
}
```

Then click **Refresh** in the MCP panel. Windsurf caps the number of tools across all servers
(100 at the time of writing): with 92 tools, `mcp-tiime` leaves little room for other servers,
so disable the tools you do not need in the MCP panel.

## Zed

Add a context server to Zed's `settings.json` (**Agent Panel → Settings → Add Custom Server**
writes the same entry):

```json
{
  "context_servers": {
    "tiime": {
      "command": "/usr/local/bin/mcp-tiime",
      "args": [],
      "env": {
        "TIIME_EMAIL": "me@example.com",
        "TIIME_PASSWORD": "..."
      }
    }
  }
}
```

Older Zed versions expect `"command": {"path": "…", "args": [], "env": {…}}` instead.

## Codex CLI

```bash
codex mcp add tiime \
  --env TIIME_EMAIL=me@example.com \
  --env TIIME_PASSWORD=... \
  -- /usr/local/bin/mcp-tiime
```

Or edit `~/.codex/config.toml`. `env_vars` forwards variables from your shell instead of
writing the password in the file:

```toml
[mcp_servers.tiime]
command = "/usr/local/bin/mcp-tiime"
env = { TIIME_EMAIL = "me@example.com" }
env_vars = ["TIIME_PASSWORD"]
```

Check the connection with `/mcp` in the Codex TUI.

## Gemini CLI

Edit `~/.gemini/settings.json` (or `.gemini/settings.json` in a project). Gemini CLI only passes
the variables listed in `env`, and expands `$VAR` from your shell:

```json
{
  "mcpServers": {
    "tiime": {
      "command": "/usr/local/bin/mcp-tiime",
      "env": {
        "TIIME_EMAIL": "me@example.com",
        "TIIME_PASSWORD": "$TIIME_PASSWORD"
      }
    }
  }
}
```

## Other clients

Any client that supports local (stdio) MCP servers works with the same three pieces of
information:

| Field | Value |
|---|---|
| Command | Absolute path to `mcp-tiime` (no arguments) |
| Transport | stdio |
| Environment | Optional `TIIME_*` variables, see [Configuration](../README.md#configuration) |

The server writes the MCP protocol on stdout and its JSON logs on stderr. It does not offer an
HTTP transport, so web-only clients that need a remote URL cannot use it.

## Running through Docker

Every client above can run the [Docker image](../README.md#docker) instead of the binary: use
`docker` as the command and pass the variables with `-e`, which forwards them from the `env`
block. With Cursor, for instance:

```json
{
  "mcpServers": {
    "tiime": {
      "command": "docker",
      "args": [
        "run", "--rm", "-i",
        "-v", "mcp-tiime:/data",
        "-e", "TIIME_EMAIL",
        "-e", "TIIME_PASSWORD",
        "ghcr.io/eko/mcp-tiime"
      ],
      "env": {
        "TIIME_EMAIL": "me@example.com",
        "TIIME_PASSWORD": "..."
      }
    }
  }
}
```

Keep `-i` (stdio) and the `/data` volume (session cache). With Docker, `upload_document` only
sees the folders mounted in the container.
