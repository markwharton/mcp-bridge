# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [v0.1.0] - 2026-04-19

### Added

- port stdio-to-HTTP MCP bridge from markwharton/mcp-bridge-go (6e0f4d6)

### Fixed

- normalize Version() output to bare semver (b56c4d2)

### Documentation

- refresh status line after source port landed (91cb3cf)
- add README ported from markwharton/mcp-bridge-go (c7d2ecf)
- add CI, codecov, Go version, and license badges (eefbb51)

### Maintenance

- initial pk setup with baseline (c674ff7)
- document project conventions and configure pk (4c38291)
- scaffold Go stack with plankit-derived Makefile and workflows (12e4e8f)
- fail make lint on gofmt drift (0e741cf)

[v0.1.0]: https://github.com/markwharton/mcp-bridge/compare/v0.0.0...v0.1.0
