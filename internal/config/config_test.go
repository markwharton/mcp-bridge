package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseTarget(t *testing.T) {
	tests := []struct {
		input string
		want  Target
		err   bool
	}{
		{"all", TargetAll, false},
		{"", TargetAll, false},
		{"desktop", TargetDesktop, false},
		{"Desktop", TargetDesktop, false},
		{"DESKTOP", TargetDesktop, false},
		{"code", TargetCode, false},
		{"Code", TargetCode, false},
		{"invalid", TargetAll, true},
	}

	for _, tt := range tests {
		got, err := ParseTarget(tt.input)
		if (err != nil) != tt.err {
			t.Errorf("ParseTarget(%q) error = %v, want error = %v", tt.input, err, tt.err)
		}
		if !tt.err && got != tt.want {
			t.Errorf("ParseTarget(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestReadConfig_NonExistentFile(t *testing.T) {
	config, err := readConfig("/nonexistent/path/config.json")
	if err != nil {
		t.Fatalf("readConfig should not error for non-existent file: %v", err)
	}
	if len(config) != 0 {
		t.Errorf("expected empty config, got %v", config)
	}
}

func TestReadConfig_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")
	os.WriteFile(path, []byte(""), 0644)

	config, err := readConfig(path)
	if err != nil {
		t.Fatalf("readConfig should not error for empty file: %v", err)
	}
	if len(config) != 0 {
		t.Errorf("expected empty config, got %v", config)
	}
}

func TestReadConfig_ValidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")
	content := `{"mcpServers":{"test":{"command":"test-cmd"}},"otherKey":"preserved"}`
	os.WriteFile(path, []byte(content), 0644)

	config, err := readConfig(path)
	if err != nil {
		t.Fatalf("readConfig error: %v", err)
	}

	if _, ok := config["mcpServers"]; !ok {
		t.Error("expected mcpServers key")
	}
	if _, ok := config["otherKey"]; !ok {
		t.Error("expected otherKey to be preserved")
	}
}

func TestWriteConfig_CreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "nested", "config.json")

	config := map[string]json.RawMessage{
		"mcpServers": json.RawMessage(`{}`),
	}

	err := writeConfig(path, config)
	if err != nil {
		t.Fatalf("writeConfig error: %v", err)
	}

	// Verify file exists and is valid JSON.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file should exist: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("written file should be valid JSON: %v", err)
	}
}

func TestWriteConfig_CreatesBackup(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")

	// Write initial content.
	os.WriteFile(path, []byte(`{"original":true}`), 0644)

	// Overwrite with new content.
	config := map[string]json.RawMessage{
		"updated": json.RawMessage(`true`),
	}
	err := writeConfig(path, config)
	if err != nil {
		t.Fatalf("writeConfig error: %v", err)
	}

	// Verify backup exists with original content.
	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("backup should exist: %v", err)
	}
	if string(backup) != `{"original":true}` {
		t.Errorf("backup content = %q, want original content", string(backup))
	}
}

func TestUpsertServer_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")

	entry := map[string]interface{}{
		"command": "mcp-bridge",
		"args":    []string{"--url", "https://example.com"},
	}

	err := upsertServer(path, "my-server", entry)
	if err != nil {
		t.Fatalf("upsertServer error: %v", err)
	}

	// Read back and verify.
	data, _ := os.ReadFile(path)
	var config map[string]interface{}
	json.Unmarshal(data, &config)

	servers := config["mcpServers"].(map[string]interface{})
	server := servers["my-server"].(map[string]interface{})
	if server["command"] != "mcp-bridge" {
		t.Errorf("command = %v, want mcp-bridge", server["command"])
	}
}

func TestUpsertServer_PreservesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")

	// Write initial config with an existing server and other fields.
	initial := `{
  "mcpServers": {
    "existing": {"command": "other-tool", "args": ["--flag"]}
  },
  "preferences": {"theme": "dark"}
}`
	os.WriteFile(path, []byte(initial), 0644)

	// Add a new server.
	entry := map[string]interface{}{"command": "mcp-bridge"}
	err := upsertServer(path, "new-server", entry)
	if err != nil {
		t.Fatalf("upsertServer error: %v", err)
	}

	// Read back and verify both servers exist and preferences preserved.
	data, _ := os.ReadFile(path)
	var config map[string]interface{}
	json.Unmarshal(data, &config)

	// Check preferences preserved.
	prefs := config["preferences"].(map[string]interface{})
	if prefs["theme"] != "dark" {
		t.Error("preferences should be preserved")
	}

	// Check both servers exist.
	servers := config["mcpServers"].(map[string]interface{})
	if _, ok := servers["existing"]; !ok {
		t.Error("existing server should be preserved")
	}
	if _, ok := servers["new-server"]; !ok {
		t.Error("new server should be added")
	}
}

func TestUpsertServer_OverwritesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")

	initial := `{"mcpServers":{"test":{"command":"old-cmd"}}}`
	os.WriteFile(path, []byte(initial), 0644)

	entry := map[string]interface{}{"command": "new-cmd"}
	upsertServer(path, "test", entry)

	data, _ := os.ReadFile(path)
	var config map[string]interface{}
	json.Unmarshal(data, &config)

	servers := config["mcpServers"].(map[string]interface{})
	server := servers["test"].(map[string]interface{})
	if server["command"] != "new-cmd" {
		t.Errorf("command = %v, want new-cmd", server["command"])
	}
}

func TestRemoveServer_Success(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")

	initial := `{"mcpServers":{"to-remove":{"command":"test"},"keep":{"command":"keep"}}}`
	os.WriteFile(path, []byte(initial), 0644)

	err := removeServer(path, "to-remove")
	if err != nil {
		t.Fatalf("removeServer error: %v", err)
	}

	data, _ := os.ReadFile(path)
	var config map[string]interface{}
	json.Unmarshal(data, &config)

	servers := config["mcpServers"].(map[string]interface{})
	if _, ok := servers["to-remove"]; ok {
		t.Error("server should be removed")
	}
	if _, ok := servers["keep"]; !ok {
		t.Error("other server should be preserved")
	}
}

func TestRemoveServer_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")

	initial := `{"mcpServers":{"other":{"command":"test"}}}`
	os.WriteFile(path, []byte(initial), 0644)

	err := removeServer(path, "nonexistent")
	if err == nil {
		t.Error("removeServer should error when server not found")
	}
}

func TestListServers(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")

	initial := `{"mcpServers":{"server-a":{"command":"cmd-a"},"server-b":{"type":"http","url":"https://example.com"}}}`
	os.WriteFile(path, []byte(initial), 0644)

	entries, err := listServers(path, "desktop")
	if err != nil {
		t.Fatalf("listServers error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Verify all entries have the right source.
	for _, entry := range entries {
		if entry.Source != "desktop" {
			t.Errorf("entry source = %q, want desktop", entry.Source)
		}
	}
}

func TestListServers_NonExistentFile(t *testing.T) {
	entries, err := listServers("/nonexistent/config.json", "desktop")
	if err != nil {
		t.Fatalf("listServers should not error for non-existent file: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestSetup_Desktop(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "claude_desktop_config.json")

	// Override the path function for testing.
	origPath := desktopConfigPathFunc
	desktopConfigPathFunc = func() (string, error) { return path, nil }
	defer func() { desktopConfigPathFunc = origPath }()

	opts := SetupOptions{
		Name:       "test-server",
		URL:        "https://example.com/mcp",
		Key:        "sk_test",
		AuthHeader: "X-API-Key",
		BinaryPath: "/usr/local/bin/mcp-bridge",
	}

	err := Setup(opts, TargetDesktop)
	if err != nil {
		t.Fatalf("Setup error: %v", err)
	}

	data, _ := os.ReadFile(path)
	var config map[string]interface{}
	json.Unmarshal(data, &config)

	servers := config["mcpServers"].(map[string]interface{})
	server := servers["test-server"].(map[string]interface{})

	if server["command"] != "/usr/local/bin/mcp-bridge" {
		t.Errorf("command = %v, want /usr/local/bin/mcp-bridge", server["command"])
	}

	args := server["args"].([]interface{})
	// Should include: --url, URL, --key, KEY, --auth-header, X-API-Key
	if len(args) != 6 {
		t.Fatalf("expected 6 args, got %d: %v", len(args), args)
	}
	if args[0] != "--url" || args[1] != "https://example.com/mcp" {
		t.Errorf("url args = %v %v", args[0], args[1])
	}
	if args[2] != "--key" || args[3] != "sk_test" {
		t.Errorf("key args = %v %v", args[2], args[3])
	}
	if args[4] != "--auth-header" || args[5] != "X-API-Key" {
		t.Errorf("auth-header args = %v %v", args[4], args[5])
	}
}

func TestSetup_Code(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".claude.json")

	origPath := codeConfigPathFunc
	codeConfigPathFunc = func() (string, error) { return path, nil }
	defer func() { codeConfigPathFunc = origPath }()

	opts := SetupOptions{
		Name:       "test-server",
		URL:        "https://example.com/mcp",
		Key:        "sk_test",
		AuthHeader: "X-API-Key",
		BinaryPath: "/usr/local/bin/mcp-bridge",
	}

	err := Setup(opts, TargetCode)
	if err != nil {
		t.Fatalf("Setup error: %v", err)
	}

	data, _ := os.ReadFile(path)
	var config map[string]interface{}
	json.Unmarshal(data, &config)

	servers := config["mcpServers"].(map[string]interface{})
	server := servers["test-server"].(map[string]interface{})

	if server["type"] != "http" {
		t.Errorf("type = %v, want http", server["type"])
	}
	if server["url"] != "https://example.com/mcp" {
		t.Errorf("url = %v, want https://example.com/mcp", server["url"])
	}

	headers := server["headers"].(map[string]interface{})
	if headers["X-API-Key"] != "sk_test" {
		t.Errorf("X-API-Key header = %v, want sk_test", headers["X-API-Key"])
	}
}

func TestSetup_CodeWithBearerAuth(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".claude.json")

	origPath := codeConfigPathFunc
	codeConfigPathFunc = func() (string, error) { return path, nil }
	defer func() { codeConfigPathFunc = origPath }()

	opts := SetupOptions{
		Name:       "test-server",
		URL:        "https://example.com/mcp",
		Key:        "sk_test",
		AuthHeader: "Authorization",
		BinaryPath: "/usr/local/bin/mcp-bridge",
	}

	err := Setup(opts, TargetCode)
	if err != nil {
		t.Fatalf("Setup error: %v", err)
	}

	data, _ := os.ReadFile(path)
	var config map[string]interface{}
	json.Unmarshal(data, &config)

	servers := config["mcpServers"].(map[string]interface{})
	server := servers["test-server"].(map[string]interface{})

	headers := server["headers"].(map[string]interface{})
	if headers["Authorization"] != "Bearer sk_test" {
		t.Errorf("Authorization header = %v, want 'Bearer sk_test'", headers["Authorization"])
	}
}

func TestSetup_DesktopOmitsDefaultAuthHeader(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")

	origPath := desktopConfigPathFunc
	desktopConfigPathFunc = func() (string, error) { return path, nil }
	defer func() { desktopConfigPathFunc = origPath }()

	opts := SetupOptions{
		Name:       "test",
		URL:        "https://example.com/mcp",
		Key:        "sk_test",
		AuthHeader: "Authorization", // Default — should not appear in args.
		BinaryPath: "mcp-bridge",
	}

	Setup(opts, TargetDesktop)

	data, _ := os.ReadFile(path)
	var config map[string]interface{}
	json.Unmarshal(data, &config)

	servers := config["mcpServers"].(map[string]interface{})
	server := servers["test"].(map[string]interface{})
	args := server["args"].([]interface{})

	// Should only have --url and --key (4 args), not --auth-header.
	if len(args) != 4 {
		t.Errorf("expected 4 args (no --auth-header for default), got %d: %v", len(args), args)
	}
}
