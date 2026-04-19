package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// DesktopConfigPath returns the path to Claude Desktop's config file.
//
// Platform paths:
//   - macOS:   ~/Library/Application Support/Claude/claude_desktop_config.json
//   - Windows: %APPDATA%\Claude\claude_desktop_config.json
//   - Linux:   ~/.config/Claude/claude_desktop_config.json
func DesktopConfigPath() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"), nil
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable not set")
		}
		return filepath.Join(appData, "Claude", "claude_desktop_config.json"), nil
	default: // linux and others
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json"), nil
	}
}

// CodeConfigPath returns the path to Claude Code's user-scoped config file.
//
// All platforms: ~/.claude.json
func CodeConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".claude.json"), nil
}
