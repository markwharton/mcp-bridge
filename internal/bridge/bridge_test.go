package bridge

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSetAuthHeader_Bearer(t *testing.T) {
	b := &Bridge{AuthHeader: "Authorization", Key: "sk_test123"}
	req, _ := http.NewRequest("POST", "http://example.com", nil)
	b.setAuthHeader(req)

	got := req.Header.Get("Authorization")
	want := "Bearer sk_test123"
	if got != want {
		t.Errorf("Authorization header = %q, want %q", got, want)
	}
}

func TestSetAuthHeader_Custom(t *testing.T) {
	b := &Bridge{AuthHeader: "X-API-Key", Key: "test_sk_abc123"}
	req, _ := http.NewRequest("POST", "http://example.com", nil)
	b.setAuthHeader(req)

	got := req.Header.Get("X-API-Key")
	want := "test_sk_abc123"
	if got != want {
		t.Errorf("X-API-Key header = %q, want %q", got, want)
	}

	// Authorization should not be set.
	if auth := req.Header.Get("Authorization"); auth != "" {
		t.Errorf("Authorization header should be empty, got %q", auth)
	}
}

func TestSetAuthHeader_EmptyKey(t *testing.T) {
	b := &Bridge{AuthHeader: "Authorization", Key: ""}
	req, _ := http.NewRequest("POST", "http://example.com", nil)
	b.setAuthHeader(req)

	if auth := req.Header.Get("Authorization"); auth != "" {
		t.Errorf("Authorization header should be empty when key is empty, got %q", auth)
	}
}

func TestProcessRequest_Success(t *testing.T) {
	responseBody := `{"jsonrpc":"2.0","result":{"tools":[]},"id":1}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method and headers.
		if r.Method != "POST" {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		if accept := r.Header.Get("Accept"); accept != "application/json, text/event-stream" {
			t.Errorf("Accept = %q, want application/json, text/event-stream", accept)
		}
		if pv := r.Header.Get("MCP-Protocol-Version"); pv != "2025-11-25" {
			t.Errorf("MCP-Protocol-Version = %q, want 2025-11-25", pv)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test_key" {
			t.Errorf("Authorization = %q, want Bearer test_key", auth)
		}

		// Verify body is the JSON-RPC message.
		body, _ := io.ReadAll(r.Body)
		var msg map[string]interface{}
		json.Unmarshal(body, &msg)
		if msg["method"] != "tools/list" {
			t.Errorf("request method = %v, want tools/list", msg["method"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(responseBody))
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	b := &Bridge{
		URL:        server.URL,
		Key:        "test_key",
		AuthHeader: "Authorization",
		Client:     server.Client(),
		Stdout:     &stdout,
		Stderr:     &stderr,
	}

	b.processRequest(`{"jsonrpc":"2.0","method":"tools/list","id":1}`)

	got := strings.TrimSpace(stdout.String())
	if got != responseBody {
		t.Errorf("stdout = %q, want %q", got, responseBody)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr should be empty, got %q", stderr.String())
	}
}

func TestProcessRequest_202Accepted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	b := &Bridge{
		URL:        server.URL,
		Key:        "test_key",
		AuthHeader: "Authorization",
		Client:     server.Client(),
		Stdout:     &stdout,
		Stderr:     &stderr,
	}

	b.processRequest(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)

	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty for 202, got %q", stdout.String())
	}
}

func TestProcessRequest_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Unauthorized"))
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	b := &Bridge{
		URL:        server.URL,
		Key:        "bad_key",
		AuthHeader: "Authorization",
		Client:     server.Client(),
		Stdout:     &stdout,
		Stderr:     &stderr,
	}

	b.processRequest(`{"jsonrpc":"2.0","method":"tools/list","id":42}`)

	// Should write a JSON-RPC error to stdout.
	var errResp map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}

	if errResp["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want 2.0", errResp["jsonrpc"])
	}
	// ID should be preserved from the request.
	if errResp["id"] != float64(42) {
		t.Errorf("id = %v, want 42", errResp["id"])
	}
	errObj := errResp["error"].(map[string]interface{})
	if errObj["code"] != float64(-32603) {
		t.Errorf("error code = %v, want -32603", errObj["code"])
	}
	msg := errObj["message"].(string)
	if !strings.Contains(msg, "401") {
		t.Errorf("error message should contain 401, got %q", msg)
	}

	// Should also log to stderr.
	if !strings.Contains(stderr.String(), "Bridge error:") {
		t.Errorf("stderr should contain 'Bridge error:', got %q", stderr.String())
	}
}

func TestProcessRequest_NilID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	b := &Bridge{
		URL:        server.URL,
		Key:        "test",
		AuthHeader: "Authorization",
		Client:     server.Client(),
		Stdout:     &stdout,
		Stderr:     &stderr,
	}

	// Notification (no id field) — error response should omit id.
	b.processRequest(`{"jsonrpc":"2.0","method":"notifications/test"}`)

	var errResp map[string]interface{}
	json.Unmarshal(stdout.Bytes(), &errResp)

	if _, hasID := errResp["id"]; hasID {
		t.Errorf("error response should not have 'id' for notifications, got %v", errResp["id"])
	}
}

func TestProcessRequest_SessionIDTracking(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First request: server returns a session ID.
			if sid := r.Header.Get("MCP-Session-Id"); sid != "" {
				t.Errorf("first request should not have MCP-Session-Id, got %q", sid)
			}
			w.Header().Set("MCP-Session-Id", "session-abc-123")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"jsonrpc":"2.0","result":{},"id":1}`))
		} else {
			// Second request: client should include the session ID.
			if sid := r.Header.Get("MCP-Session-Id"); sid != "session-abc-123" {
				t.Errorf("second request MCP-Session-Id = %q, want session-abc-123", sid)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"jsonrpc":"2.0","result":{},"id":2}`))
		}
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	b := &Bridge{
		URL:        server.URL,
		Key:        "test",
		AuthHeader: "Authorization",
		Client:     server.Client(),
		Stdout:     &stdout,
		Stderr:     &stderr,
	}

	b.processRequest(`{"jsonrpc":"2.0","method":"initialize","id":1}`)
	b.processRequest(`{"jsonrpc":"2.0","method":"tools/list","id":2}`)

	if callCount != 2 {
		t.Errorf("expected 2 requests, got %d", callCount)
	}
}

func TestProcessRequest_CustomAuthHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify X-API-Key is set, Authorization is not.
		if apiKey := r.Header.Get("X-API-Key"); apiKey != "test_sk_test" {
			t.Errorf("X-API-Key = %q, want test_sk_test", apiKey)
		}
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("Authorization should be empty, got %q", auth)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"jsonrpc":"2.0","result":{},"id":1}`))
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	b := &Bridge{
		URL:        server.URL,
		Key:        "test_sk_test",
		AuthHeader: "X-API-Key",
		Client:     server.Client(),
		Stdout:     &stdout,
		Stderr:     &stderr,
	}

	b.processRequest(`{"jsonrpc":"2.0","method":"tools/list","id":1}`)

	if stderr.Len() != 0 {
		t.Errorf("stderr should be empty, got %q", stderr.String())
	}
}

func TestRun_MultipleMessages(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		body, _ := io.ReadAll(r.Body)
		var msg map[string]interface{}
		json.Unmarshal(body, &msg)
		id := msg["id"]
		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{"jsonrpc": "2.0", "result": map[string]interface{}{}, "id": id}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	input := `{"jsonrpc":"2.0","method":"initialize","id":1}
{"jsonrpc":"2.0","method":"tools/list","id":2}

{"jsonrpc":"2.0","method":"tools/call","id":3}
`

	var stdout, stderr bytes.Buffer
	b := &Bridge{
		URL:        server.URL,
		Key:        "test",
		AuthHeader: "Authorization",
		Client:     server.Client(),
		Stdin:      strings.NewReader(input),
		Stdout:     &stdout,
		Stderr:     &stderr,
	}

	err := b.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if callCount != 3 {
		t.Errorf("expected 3 HTTP requests, got %d", callCount)
	}

	// Should have 3 response lines (blank lines in input are skipped).
	// Filter empty lines since json.Encoder adds trailing newlines.
	allLines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	var lines []string
	for _, l := range allLines {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) != 3 {
		t.Errorf("expected 3 response lines, got %d: %v", len(lines), lines)
	}
}

func TestWriteError_WithStringID(t *testing.T) {
	var stdout, stderr bytes.Buffer
	b := &Bridge{Stdout: &stdout, Stderr: &stderr}

	b.writeError("req-1", "something failed")

	var errResp map[string]interface{}
	json.Unmarshal(stdout.Bytes(), &errResp)

	if errResp["id"] != "req-1" {
		t.Errorf("id = %v, want req-1", errResp["id"])
	}
	if errResp["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want 2.0", errResp["jsonrpc"])
	}
}

func TestWriteError_WithoutID(t *testing.T) {
	var stdout, stderr bytes.Buffer
	b := &Bridge{Stdout: &stdout, Stderr: &stderr}

	b.writeError(nil, "something failed")

	var errResp map[string]interface{}
	json.Unmarshal(stdout.Bytes(), &errResp)

	if _, hasID := errResp["id"]; hasID {
		t.Errorf("should not have id field, got %v", errResp["id"])
	}
}
