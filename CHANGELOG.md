# Changelog

All notable changes to the AIVory Monitor Go Agent will be documented in this file.

This project adheres to [Semantic Versioning](https://semver.org/).

## [0.1.0] - 2026-02-16

### Added
- Panic recovery via defer/recover pattern
- Manual error capture with optional context
- Goroutine-safe agent with sync.RWMutex
- Functional options configuration pattern
- WebSocket transport via gorilla/websocket
- SIGINT/SIGTERM signal handling for graceful shutdown
- User and custom context enrichment
- Configurable sampling rate and capture depth
- Environment variable and programmatic configuration
