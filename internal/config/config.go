// Package config manages Claude Desktop and Claude Code MCP server configurations.
//
// It reads and writes configuration files for both clients, preserving existing
// entries and unknown fields. Writes are atomic (temp file + rename) with backups.
package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Path functions are package-level variables so tests can override them.
var (
	desktopConfigPathFunc = DesktopConfigPath
	codeConfigPathFunc    = CodeConfigPath
)

// Target specifies which Claude client(s) to configure.
type Target int

const (
	TargetAll     Target = iota // Both Desktop and Code
	TargetDesktop               // Claude Desktop only
	TargetCode                  // Claude Code only
)

// ParseTarget converts a string to a Target value.
func ParseTarget(s string) (Target, error) {
	switch strings.ToLower(s) {
	case "all", "":
		return TargetAll, nil
	case "desktop":
		return TargetDesktop, nil
	case "code":
		return TargetCode, nil
	default:
		return TargetAll, fmt.Errorf("invalid target %q (use: desktop, code, all)", s)
	}
}

// ServerEntry represents an MCP server entry found in a config file.
type ServerEntry struct {
	Name   string // Server name (key in mcpServers)
	Source string // "desktop" or "code"
	Raw    json.RawMessage
}

// SetupOptions contains the parameters for setting up an MCP server.
type SetupOptions struct {
	Name       string // Server name in config
	URL        string // MCP endpoint URL
	Key        string // API key or Bearer token
	AuthHeader string // HTTP header name for auth (e.g., "Authorization", "X-API-Key")
	BinaryPath string // Path to the mcp-bridge binary
}

// Setup configures an MCP server in both Claude Desktop and Claude Code.
//
// For Claude Desktop, it adds a stdio bridge entry that launches mcp-bridge.
// For Claude Code, it adds a direct HTTP entry (no bridge needed).
func Setup(opts SetupOptions, target Target) error {
	if target == TargetAll || target == TargetDesktop {
		if err := setupDesktop(opts); err != nil {
			return fmt.Errorf("desktop config: %w", err)
		}
	}
	if target == TargetAll || target == TargetCode {
		if err := setupCode(opts); err != nil {
			return fmt.Errorf("code config: %w", err)
		}
	}
	return nil
}

func setupDesktop(opts SetupOptions) error {
	configPath, err := desktopConfigPathFunc()
	if err != nil {
		return err
	}

	// Build args list for the bridge command.
	args := []interface{}{"--url", opts.URL}
	if opts.Key != "" {
		args = append(args, "--key", opts.Key)
	}
	if opts.AuthHeader != "" && opts.AuthHeader != "Authorization" {
		args = append(args, "--auth-header", opts.AuthHeader)
	}

	entry := map[string]interface{}{
		"command": opts.BinaryPath,
		"args":    args,
	}

	return upsertServer(configPath, opts.Name, entry)
}

func setupCode(opts SetupOptions) error {
	configPath, err := codeConfigPathFunc()
	if err != nil {
		return err
	}

	entry := map[string]interface{}{
		"type": "http",
		"url":  opts.URL,
	}

	// Build headers map for auth.
	if opts.Key != "" {
		headers := map[string]string{}
		authHeader := opts.AuthHeader
		if authHeader == "" {
			authHeader = "Authorization"
		}
		if authHeader == "Authorization" {
			headers["Authorization"] = "Bearer " + opts.Key
		} else {
			headers[authHeader] = opts.Key
		}
		entry["headers"] = headers
	}

	return upsertServer(configPath, opts.Name, entry)
}

// Remove removes an MCP server from configuration files.
func Remove(name string, target Target) error {
	var errs []string

	if target == TargetAll || target == TargetDesktop {
		configPath, err := desktopConfigPathFunc()
		if err != nil {
			errs = append(errs, fmt.Sprintf("desktop: %s", err))
		} else if err := removeServer(configPath, name); err != nil {
			errs = append(errs, fmt.Sprintf("desktop: %s", err))
		}
	}

	if target == TargetAll || target == TargetCode {
		configPath, err := codeConfigPathFunc()
		if err != nil {
			errs = append(errs, fmt.Sprintf("code: %s", err))
		} else if err := removeServer(configPath, name); err != nil {
			errs = append(errs, fmt.Sprintf("code: %s", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// List returns all MCP server entries from configuration files.
func List(target Target) ([]ServerEntry, error) {
	var entries []ServerEntry

	if target == TargetAll || target == TargetDesktop {
		configPath, err := desktopConfigPathFunc()
		if err == nil {
			if e, err := listServers(configPath, "desktop"); err == nil {
				entries = append(entries, e...)
			}
		}
	}

	if target == TargetAll || target == TargetCode {
		configPath, err := codeConfigPathFunc()
		if err == nil {
			if e, err := listServers(configPath, "code"); err == nil {
				entries = append(entries, e...)
			}
		}
	}

	return entries, nil
}

// readConfig reads a Claude config file, returning the full JSON as an ordered map.
// Returns an empty map if the file doesn't exist.
func readConfig(path string) (map[string]json.RawMessage, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	// Handle empty files.
	if len(data) == 0 {
		return map[string]json.RawMessage{}, nil
	}

	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	return config, nil
}

// writeConfig writes a Claude config file atomically (temp file + rename).
// Creates a backup of the existing file if it exists.
func writeConfig(path string, config map[string]json.RawMessage) error {
	// Ensure parent directory exists.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	// Create backup if file exists.
	if _, err := os.Stat(path); err == nil {
		backupPath := path + ".bak"
		data, err := os.ReadFile(path)
		if err == nil {
			os.WriteFile(backupPath, data, 0644)
		}
	}

	// Marshal with pretty printing.
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	data = append(data, '\n')

	// Write atomically via temp file + rename.
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// getMCPServers extracts the mcpServers map from a config.
func getMCPServers(config map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	raw, ok := config["mcpServers"]
	if !ok {
		return map[string]json.RawMessage{}, nil
	}

	var servers map[string]json.RawMessage
	if err := json.Unmarshal(raw, &servers); err != nil {
		return nil, fmt.Errorf("parsing mcpServers: %w", err)
	}

	return servers, nil
}

// setMCPServers writes the mcpServers map back into a config.
func setMCPServers(config map[string]json.RawMessage, servers map[string]json.RawMessage) error {
	data, err := json.Marshal(servers)
	if err != nil {
		return err
	}
	config["mcpServers"] = json.RawMessage(data)
	return nil
}

// upsertServer adds or updates a server entry in a config file.
func upsertServer(path, name string, entry interface{}) error {
	config, err := readConfig(path)
	if err != nil {
		return err
	}

	servers, err := getMCPServers(config)
	if err != nil {
		return err
	}

	entryData, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling server entry: %w", err)
	}

	servers[name] = json.RawMessage(entryData)
	if err := setMCPServers(config, servers); err != nil {
		return err
	}

	return writeConfig(path, config)
}

// removeServer removes a server entry from a config file.
func removeServer(path, name string) error {
	config, err := readConfig(path)
	if err != nil {
		return err
	}

	servers, err := getMCPServers(config)
	if err != nil {
		return err
	}

	if _, ok := servers[name]; !ok {
		return fmt.Errorf("server %q not found", name)
	}

	delete(servers, name)
	if err := setMCPServers(config, servers); err != nil {
		return err
	}

	return writeConfig(path, config)
}

// listServers lists all server entries from a config file.
func listServers(path, source string) ([]ServerEntry, error) {
	config, err := readConfig(path)
	if err != nil {
		return nil, err
	}

	servers, err := getMCPServers(config)
	if err != nil {
		return nil, err
	}

	var entries []ServerEntry
	for name, raw := range servers {
		entries = append(entries, ServerEntry{
			Name:   name,
			Source: source,
			Raw:    raw,
		})
	}

	return entries, nil
}
