# mcp-bridge

[![CI](https://github.com/markwharton/mcp-bridge/actions/workflows/ci.yml/badge.svg)](https://github.com/markwharton/mcp-bridge/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/markwharton/mcp-bridge/graph/badge.svg?token=pIUP6YrtKk)](https://codecov.io/gh/markwharton/mcp-bridge)
[![Go](https://img.shields.io/badge/Go-1.21-00ADD8.svg)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A lightweight Go binary that bridges stdio-based MCP clients (like Claude Desktop) to remote HTTP-based MCP servers. Single binary, zero dependencies, ~5.5MB.

## Why?

Claude Desktop only supports stdio transport for MCP servers. If your MCP server runs over HTTP ([Streamable HTTP](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports#streamable-http)), you need a bridge. Existing bridges require Node.js or Python runtimes. `mcp-bridge` is a single Go binary with no runtime dependencies.

Claude Code supports HTTP transport natively, but has [known issues](https://github.com/anthropics/claude-code/issues/7290) with authentication headers and connection timeouts. The bridge provides a reliable alternative for both clients.

## Quick Start

### 1. Install

**Homebrew (recommended on macOS):**

```bash
brew install markwharton/plankit/mcp-bridge
```

**Pre-built binaries:** Download from [Releases](https://github.com/markwharton/mcp-bridge/releases), then:

```bash
# macOS / Linux: make executable
chmod +x mcp-bridge-*

# macOS only: remove quarantine (unsigned binary)
xattr -d com.apple.quarantine mcp-bridge-*

# Move to PATH (rename to mcp-bridge)
sudo mv mcp-bridge-darwin-arm64 /usr/local/bin/mcp-bridge
```

**Build from source:**
```bash
go install github.com/markwharton/mcp-bridge/cmd/mcp-bridge@latest
```

**Or clone and build:**
```bash
git clone https://github.com/markwharton/mcp-bridge.git
cd mcp-bridge
make install
```

### 2. Configure Claude

One command configures both Claude Desktop (stdio bridge) and Claude Code (direct HTTP):

```bash
mcp-bridge setup --name my-server \
  --url https://example.com/api/mcp \
  --key your_api_key
```

### 3. Restart Claude

Restart Claude Desktop and/or Claude Code to pick up the new configuration.

## Usage

### Bridge Mode (default)

```bash
mcp-bridge --url <endpoint> [--key <api-key>] [--auth-header <header>] [--timeout <seconds>]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--url` | MCP endpoint URL (required) | - |
| `--key` | API key or Bearer token | - |
| `--auth-header` | HTTP header name for auth | `Authorization` |
| `--timeout` | HTTP request timeout in seconds | `30` |

### Setup (configure Claude clients)

```bash
mcp-bridge setup --name <server-name> --url <endpoint> [--key <api-key>] [--auth-header <header>] [--target <desktop|code|all>]
```

Configures both Claude Desktop and Claude Code in one command:
- **Claude Desktop**: Adds a stdio bridge entry (launches `mcp-bridge` as a child process)
- **Claude Code**: Adds a direct HTTP entry (no bridge needed, connects directly)

### Config Management

```bash
mcp-bridge config list [--target <desktop|code|all>]
mcp-bridge config remove --name <server-name> [--target <desktop|code|all>]
```

### Version

```bash
mcp-bridge version     # or --version, -v
```

## Authentication

The `--auth-header` flag sets the HTTP header **name**, and `--key` sets the **value**.

**Standard (default):** Sends `Authorization: Bearer <key>` per the [MCP spec](https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization).
```bash
mcp-bridge --url https://example.com/mcp --key sk_abc123
# Sends: Authorization: Bearer sk_abc123
```

**Custom header:** For platforms that use non-standard auth headers (e.g., Azure Static Web Apps strips `Authorization`):
```bash
mcp-bridge --url https://example.com/mcp --key sk_abc123 --auth-header X-API-Key
# Sends: X-API-Key: sk_abc123
```

## MCP Specification

This bridge implements the [MCP Specification (2025-11-25)](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports), translating between two official transports:

### stdio Transport (client side)

What Claude Desktop speaks. Newline-delimited JSON-RPC 2.0 messages on stdin/stdout.

### Streamable HTTP Transport (server side)

The bridge sends each JSON-RPC message as an HTTP POST with the required headers:
- `Content-Type: application/json`
- `Accept: application/json, text/event-stream`
- `MCP-Protocol-Version: 2025-11-25`
- `MCP-Session-Id` (tracked from server responses, included on subsequent requests)

### Stateless JSON Mode vs Full Streaming

The Streamable HTTP spec supports two response modes:

| Mode | Response Type | Server-Initiated Messages | Platform Compatibility |
|------|--------------|--------------------------|----------------------|
| **Stateless JSON** | `application/json` | No | All (Azure Functions, AWS Lambda, serverless) |
| **Full SSE Streaming** | `text/event-stream` | Yes | Dedicated servers, containers |

**Stateless JSON mode** is enabled server-side via `enableJsonResponse: true` in the MCP SDK. This is the recommended approach for serverless platforms, which have hard timeouts on HTTP connections (e.g., Azure Functions: 230 seconds) that make SSE streaming impractical.

```typescript
// Server-side example (TypeScript MCP SDK)
const transport = new WebStandardStreamableHTTPServerTransport({
  sessionIdGenerator: undefined,  // Stateless - no session tracking
  enableJsonResponse: true,       // JSON responses, no SSE streaming
});
```

The bridge currently targets **stateless JSON mode**. SSE streaming support is planned for a future release.

## How It Looks in Practice

### Claude Desktop (`claude_desktop_config.json`)

```json
{
  "mcpServers": {
    "my-server": {
      "command": "mcp-bridge",
      "args": [
        "--url", "https://example.com/api/mcp",
        "--key", "your_api_key"
      ]
    }
  }
}
```

Config file location:
- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
- **Linux**: `~/.config/Claude/claude_desktop_config.json`

### Claude Code (`~/.claude.json`)

```json
{
  "mcpServers": {
    "my-server": {
      "type": "http",
      "url": "https://example.com/api/mcp",
      "headers": {
        "Authorization": "Bearer your_api_key"
      }
    }
  }
}
```

## How It Works

```
Claude Desktop                    mcp-bridge                      MCP Server
     │                                │                                │
     │  JSON-RPC via stdin            │                                │
     ├───────────────────────────────>│                                │
     │                                │  HTTP POST + JSON-RPC body     │
     │                                ├───────────────────────────────>│
     │                                │                                │
     │                                │  HTTP 200 + JSON-RPC response  │
     │                                │<───────────────────────────────┤
     │  JSON-RPC via stdout           │                                │
     │<───────────────────────────────┤                                │
```

1. Claude Desktop launches `mcp-bridge` as a child process
2. `mcp-bridge` reads JSON-RPC 2.0 messages line-by-line from stdin
3. Each message is POSTed to the configured HTTP endpoint with auth headers
4. HTTP responses are written back to stdout
5. Errors are logged to stderr and returned as JSON-RPC errors

## Platform Support

| Platform | Architecture | Binary |
|----------|-------------|--------|
| macOS | Intel (amd64) | `mcp-bridge-darwin-amd64` |
| macOS | Apple Silicon (arm64) | `mcp-bridge-darwin-arm64` |
| Linux | x86_64 (amd64) | `mcp-bridge-linux-amd64` |
| Linux | ARM64 | `mcp-bridge-linux-arm64` |
| Windows | x86_64 (amd64) | `mcp-bridge-windows-amd64.exe` |

## Troubleshooting

### "Permission denied" (macOS / Linux)

Downloaded binaries need the executable bit set:
```bash
chmod +x mcp-bridge-*
```

### "Cannot be opened" or quarantine warning (macOS)

macOS Gatekeeper blocks unsigned binaries. Remove the quarantine attribute:
```bash
xattr -d com.apple.quarantine mcp-bridge-*
```

### "Unauthorized" or "401" Errors

1. Verify your API key is correct
2. Check the auth header: some platforms use `X-API-Key` instead of `Authorization`
3. Test directly with curl:
   ```bash
   curl -X POST https://your-server.com/api/mcp \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer your_key" \
     -d '{"jsonrpc":"2.0","method":"tools/list","id":1}'
   ```

### Connection Timeouts

Increase the timeout for slow connections or cold-start scenarios:
```bash
mcp-bridge --url ... --key ... --timeout 60
```

### Debug Output

Errors are logged to stderr:
```bash
mcp-bridge --url ... --key ... 2>debug.log
```

## Development

```bash
make build       # Build for current platform
make build-all   # Cross-compile for all platforms
make test        # Run tests
make fmt         # Format code
make lint        # Run go vet
make install     # Install to GOPATH/bin
```

## Future Enhancements

- **Full SSE streaming** - Parse `text/event-stream` responses for servers using streaming mode
- **Claude Code `.mcp.json` support** - Project-scoped config management

## License

MIT - see [LICENSE](LICENSE).
