// Package bridge implements the core stdio-to-HTTP MCP bridge.
//
// It reads JSON-RPC 2.0 messages from stdin (MCP stdio transport) and forwards
// them as HTTP POST requests to a remote MCP endpoint (Streamable HTTP transport),
// writing responses back to stdout.
//
// This implements the client side of both MCP transports as defined in the
// MCP Specification (2025-11-25):
// https://modelcontextprotocol.io/specification/2025-11-25/basic/transports
package bridge

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
)

// mcpProtocolVersion is the MCP protocol version sent on all requests.
// See: https://modelcontextprotocol.io/specification/2025-11-25/basic/transports#protocol-version-header
const mcpProtocolVersion = "2025-11-25"

// maxScanSize is the maximum buffer size for reading JSON-RPC messages from stdin.
const maxScanSize = 10 * 1024 * 1024 // 10MB

// Bridge translates between MCP stdio and Streamable HTTP transports.
type Bridge struct {
	// URL is the remote MCP endpoint URL (required).
	URL string

	// Key is the API key or Bearer token value.
	Key string

	// AuthHeader is the HTTP header name for authentication.
	// When set to "Authorization", the key is sent as "Bearer <key>".
	// For any other header name, the key is sent as-is.
	AuthHeader string

	// Client is the HTTP client used for requests.
	Client *http.Client

	// Stdin is the input reader. Defaults to os.Stdin.
	Stdin io.Reader

	// Stdout is the output writer. Defaults to os.Stdout.
	Stdout io.Writer

	// Stderr is the error/log writer. Defaults to os.Stderr.
	Stderr io.Writer

	// sessionID tracks the MCP-Session-Id from the server.
	// See: https://modelcontextprotocol.io/specification/2025-11-25/basic/transports#session-management
	sessionID string

	// writeMu protects concurrent writes to stdout.
	writeMu sync.Mutex
}

func (b *Bridge) stdin() io.Reader {
	if b.Stdin != nil {
		return b.Stdin
	}
	return os.Stdin
}

func (b *Bridge) stdout() io.Writer {
	if b.Stdout != nil {
		return b.Stdout
	}
	return os.Stdout
}

func (b *Bridge) stderr() io.Writer {
	if b.Stderr != nil {
		return b.Stderr
	}
	return os.Stderr
}

// Run starts the bridge, reading from stdin until EOF.
func (b *Bridge) Run() error {
	scanner := bufio.NewScanner(b.stdin())
	buf := make([]byte, maxScanSize)
	scanner.Buffer(buf, maxScanSize)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Process requests sequentially to maintain order.
		// MCP protocol expects responses in order for some operations.
		b.processRequest(line)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stdin: %w", err)
	}

	return nil
}

func (b *Bridge) processRequest(msg string) {
	// Extract request ID for error responses.
	var req map[string]interface{}
	var requestID interface{}
	if err := json.Unmarshal([]byte(msg), &req); err == nil {
		requestID = req["id"]
	}

	// Create HTTP request.
	httpReq, err := http.NewRequest("POST", b.URL, bytes.NewBufferString(msg))
	if err != nil {
		b.writeError(requestID, fmt.Sprintf("Failed to create request: %s", err.Error()))
		return
	}

	// Set required MCP Streamable HTTP headers.
	// See: https://modelcontextprotocol.io/specification/2025-11-25/basic/transports#sending-messages-to-the-server
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set("MCP-Protocol-Version", mcpProtocolVersion)

	// Set authentication header.
	b.setAuthHeader(httpReq)

	// Include session ID if the server provided one.
	if b.sessionID != "" {
		httpReq.Header.Set("MCP-Session-Id", b.sessionID)
	}

	// Execute request.
	resp, err := b.Client.Do(httpReq)
	if err != nil {
		b.writeError(requestID, fmt.Sprintf("HTTP request failed: %s", err.Error()))
		return
	}
	defer resp.Body.Close()

	// Track session ID from server response.
	if sid := resp.Header.Get("MCP-Session-Id"); sid != "" {
		b.sessionID = sid
	}

	// Read response body.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		b.writeError(requestID, fmt.Sprintf("Failed to read response: %s", err.Error()))
		return
	}

	// Check for HTTP errors (200 OK and 202 Accepted are both valid).
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		b.writeError(requestID, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
		return
	}

	// For 202 Accepted (notifications), don't write any response.
	if resp.StatusCode == http.StatusAccepted {
		return
	}

	// Write successful response atomically.
	b.writeMu.Lock()
	fmt.Fprintln(b.stdout(), string(body))
	b.writeMu.Unlock()
}

// setAuthHeader sets the authentication header on the HTTP request.
func (b *Bridge) setAuthHeader(req *http.Request) {
	if b.Key == "" {
		return
	}
	if b.AuthHeader == "Authorization" {
		req.Header.Set("Authorization", "Bearer "+b.Key)
	} else {
		req.Header.Set(b.AuthHeader, b.Key)
	}
}

// writeError writes a JSON-RPC error response to stdout and logs to stderr.
func (b *Bridge) writeError(id interface{}, message string) {
	errResp := map[string]interface{}{
		"jsonrpc": "2.0",
		"error": map[string]interface{}{
			"code":    -32603,
			"message": message,
		},
	}

	// Only include id if it's a valid string or number (MCP SDK rejects null).
	if id != nil {
		errResp["id"] = id
	}

	b.writeMu.Lock()
	defer b.writeMu.Unlock()

	json.NewEncoder(b.stdout()).Encode(errResp)
	fmt.Fprintln(b.stderr(), "Bridge error:", message)
}
