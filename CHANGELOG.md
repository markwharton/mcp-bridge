# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [v0.2.0] - 2026-06-12

### Added

- add maintainer skill for plankit.com notes prompts (a218285)

### Documentation

- add Homebrew install and fix stale tap reference (89c3a60)

### Maintenance

- regenerate pk-managed files (ebf4928)
- bump codecov/codecov-action from 5.5.4 to 6.0.0 (169bc26)
- regenerate pk-managed files for v0.13.0 (9f6193f)
- bump codecov/codecov-action from 6.0.0 to 6.0.1 (6a07e13)
- bump codecov/codecov-action from 6.0.1 to 7.0.0 (ae7a089)
- bump actions/checkout from 6.0.2 to 6.0.3 (0fe2ff0)
- update pk-managed files for v0.24.0 (c998407)

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
[v0.2.0]: https://github.com/markwharton/mcp-bridge/compare/v0.1.0...v0.2.0
