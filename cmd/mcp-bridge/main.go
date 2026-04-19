// MCP Bridge - A lightweight stdio-to-HTTP bridge for MCP.
//
// Connects Claude Desktop (or other stdio-based MCP clients) to remote
// HTTP-based MCP servers using the Streamable HTTP transport.
//
// Usage:
//
//	mcp-bridge --url https://example.com/api/mcp --key your_api_key
//
// See https://modelcontextprotocol.io/specification/2025-11-25/basic/transports
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/markwharton/mcp-bridge/internal/bridge"
	"github.com/markwharton/mcp-bridge/internal/config"
	"github.com/markwharton/mcp-bridge/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "setup":
		runSetup(os.Args[2:])
	case "config":
		if len(os.Args) < 3 {
			printConfigUsage()
			os.Exit(1)
		}
		switch os.Args[2] {
		case "list":
			runConfigList(os.Args[3:])
		case "remove":
			runConfigRemove(os.Args[3:])
		default:
			fmt.Fprintf(os.Stderr, "Unknown config command: %s\n\n", os.Args[2])
			printConfigUsage()
			os.Exit(1)
		}
	case "version", "--version", "-v":
		fmt.Fprintf(os.Stderr, "mcp-bridge %s\n", version.Version())
	case "help", "--help", "-h":
		printUsage()
	default:
		// If first arg looks like a flag, treat as implicit bridge mode.
		if strings.HasPrefix(os.Args[1], "-") {
			runBridge(os.Args[1:])
		} else {
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
			printUsage()
			os.Exit(1)
		}
	}
}

func runBridge(args []string) {
	fs := flag.NewFlagSet("bridge", flag.ExitOnError)
	url := fs.String("url", "", "MCP endpoint URL (required)")
	key := fs.String("key", "", "API key or Bearer token")
	authHeader := fs.String("auth-header", "Authorization", "HTTP header name for auth")
	timeout := fs.Int("timeout", 30, "HTTP request timeout in seconds")
	fs.Parse(args)

	if *url == "" {
		fmt.Fprintln(os.Stderr, "mcp-bridge - stdio-to-HTTP bridge for MCP")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Usage: mcp-bridge --url <endpoint> [--key <api-key>] [--auth-header <header>] [--timeout <seconds>]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  --url          MCP endpoint URL (required)")
		fmt.Fprintln(os.Stderr, "  --key          API key or Bearer token")
		fmt.Fprintln(os.Stderr, "  --auth-header  HTTP header name for auth (default: Authorization)")
		fmt.Fprintln(os.Stderr, "                 Default sends: Authorization: Bearer <key>")
		fmt.Fprintln(os.Stderr, "                 Custom sends:  <header>: <key>")
		fmt.Fprintln(os.Stderr, "  --timeout      HTTP request timeout in seconds (default: 30)")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Example:")
		fmt.Fprintln(os.Stderr, "  mcp-bridge --url https://example.com/api/mcp --key sk_abc123")
		fmt.Fprintln(os.Stderr, "  mcp-bridge --url https://example.com/api/mcp --key sk_abc123 --auth-header X-API-Key")
		os.Exit(1)
	}

	b := &bridge.Bridge{
		URL:        *url,
		Key:        *key,
		AuthHeader: *authHeader,
		Client: &http.Client{
			Timeout: time.Duration(*timeout) * time.Second,
		},
	}

	if err := b.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runSetup(args []string) {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	name := fs.String("name", "", "Server name in config (required)")
	url := fs.String("url", "", "MCP endpoint URL (required)")
	key := fs.String("key", "", "API key or Bearer token")
	authHeader := fs.String("auth-header", "Authorization", "HTTP header name for auth")
	target := fs.String("target", "all", "Target client: desktop, code, all")
	fs.Parse(args)

	if *name == "" || *url == "" {
		fmt.Fprintln(os.Stderr, "Usage: mcp-bridge setup --name <server-name> --url <endpoint> [--key <api-key>] [--auth-header <header>] [--target <desktop|code|all>]")
		os.Exit(1)
	}

	t, err := config.ParseTarget(*target)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	// Auto-detect binary path.
	binaryPath, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Warning: could not detect binary path, using 'mcp-bridge'")
		binaryPath = "mcp-bridge"
	}

	opts := config.SetupOptions{
		Name:       *name,
		URL:        *url,
		Key:        *key,
		AuthHeader: *authHeader,
		BinaryPath: binaryPath,
	}

	if err := config.Setup(opts, t); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	switch t {
	case config.TargetDesktop:
		fmt.Fprintf(os.Stderr, "Added %q to Claude Desktop config.\n", *name)
	case config.TargetCode:
		fmt.Fprintf(os.Stderr, "Added %q to Claude Code config.\n", *name)
	default:
		fmt.Fprintf(os.Stderr, "Added %q to Claude Desktop and Claude Code configs.\n", *name)
	}
	fmt.Fprintln(os.Stderr, "Restart your Claude client(s) to apply changes.")
}

func runConfigList(args []string) {
	fs := flag.NewFlagSet("config list", flag.ExitOnError)
	target := fs.String("target", "all", "Target client: desktop, code, all")
	fs.Parse(args)

	t, err := config.ParseTarget(*target)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	entries, err := config.List(t)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "No MCP servers configured.")
		return
	}

	for _, entry := range entries {
		// Parse the entry to show a summary.
		var parsed map[string]interface{}
		json.Unmarshal(entry.Raw, &parsed)

		summary := ""
		if cmd, ok := parsed["command"].(string); ok {
			summary = cmd
			if entryArgs, ok := parsed["args"].([]interface{}); ok {
				parts := make([]string, len(entryArgs))
				for i, a := range entryArgs {
					parts[i] = fmt.Sprintf("%v", a)
				}
				summary += " " + strings.Join(parts, " ")
			}
		} else if url, ok := parsed["url"].(string); ok {
			entryType, _ := parsed["type"].(string)
			summary = fmt.Sprintf("[%s] %s", entryType, url)
		}

		fmt.Fprintf(os.Stderr, "  %-10s %-12s %s\n", entry.Source, entry.Name, summary)
	}
}

func runConfigRemove(args []string) {
	fs := flag.NewFlagSet("config remove", flag.ExitOnError)
	name := fs.String("name", "", "Server name to remove (required)")
	target := fs.String("target", "all", "Target client: desktop, code, all")
	fs.Parse(args)

	if *name == "" {
		fmt.Fprintln(os.Stderr, "Usage: mcp-bridge config remove --name <server-name> [--target <desktop|code|all>]")
		os.Exit(1)
	}

	t, err := config.ParseTarget(*target)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	if err := config.Remove(*name, t); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Removed %q from config.\n", *name)
	fmt.Fprintln(os.Stderr, "Restart your Claude client(s) to apply changes.")
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "mcp-bridge - stdio-to-HTTP bridge for MCP")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "A lightweight Go binary that connects stdio-based MCP clients (like Claude Desktop)")
	fmt.Fprintln(os.Stderr, "to remote HTTP-based MCP servers. Single binary, zero dependencies.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  mcp-bridge --url <endpoint> [--key <api-key>]     Run bridge (default)")
	fmt.Fprintln(os.Stderr, "  mcp-bridge setup --name <name> --url <endpoint>   Configure Claude clients")
	fmt.Fprintln(os.Stderr, "  mcp-bridge config list                            List configured servers")
	fmt.Fprintln(os.Stderr, "  mcp-bridge config remove --name <name>            Remove a server")
	fmt.Fprintln(os.Stderr, "  mcp-bridge version                                Print version")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Spec: https://modelcontextprotocol.io/specification/2025-11-25/basic/transports")
}

func printConfigUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  mcp-bridge config list [--target <desktop|code|all>]")
	fmt.Fprintln(os.Stderr, "  mcp-bridge config remove --name <name> [--target <desktop|code|all>]")
}
