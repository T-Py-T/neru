# AGENTS.md - Neru Development Guide

Neru is a keyboard-driven navigation tool for macOS built with Go and Objective-C.

## Domain Concepts

- **Mode**: Navigation context (hints, grid, scroll, action)
- **Bridge**: Objective-C macOS integration layer
- **Adapter**: Port implementation for external systems
- **Port**: Interface definition for system capabilities (e.g., [accessibility.go](internal/core/ports/accessibility.go))

## Architecture & Cross-Platform

Neru follows a **Hexagonal Architecture (Ports and Adapters)**. All OS-specific code is strictly isolated.

### The "One Rule"

**Non-darwin-tagged code must never import `internal/core/infra/platform/darwin`.** This is enforced by `golangci-lint` using `depguard`.

### File Organization for Platforms

- **Ports**: [internal/core/ports/](internal/core/ports/)
- **Infrastructure**: [internal/core/infra/](internal/core/infra/)
- **Platform Factory**: [internal/core/infra/platform/factory.go](internal/core/infra/platform/factory.go) and build-tagged siblings.
- **Platform Implementations**: [internal/core/infra/platform/darwin/](internal/core/infra/platform/darwin/), `linux/`, `windows/`.

## AI Assistant Exploration Tips

### Finding the "Source of Truth"

- **App Startup**: [app_initialization.go](internal/app/app_initialization.go)
- **Navigation Logic**: [internal/app/modes/](internal/app/modes/)
- **Coordinate Conversion**: [conversion.go](internal/ui/coordinates/conversion.go)
- **Error Definitions**: [errors.go](internal/core/errors/errors.go)
- **Native macOS Logic**: [internal/core/infra/platform/darwin/](internal/core/infra/platform/darwin/)

### Contextual Shortcuts

- To understand **Mode** behavior: Read `internal/app/modes/base.go` and `handler.go`.
- To understand **Accessibility**: Read `internal/core/ports/accessibility.go` (Port) and `internal/core/infra/accessibility/adapter.go` (Adapter).
- To understand **Overlay** rendering: Read `internal/core/ports/overlay.go` and `internal/app/components/overlayutil/factory_darwin.go`.

## Documentation

Documentation is progressively disclosed. Start here, then navigate to detailed docs:

- [System Architecture](./docs/ARCHITECTURE.md) - Comprehensive architecture overview
- [Development Guide](./docs/DEVELOPMENT.md) - Build, testing, architecture
- [Coding Standards](./docs/CODING_STANDARDS.md) - Go & Objective-C conventions
- [CLI Usage](./docs/CLI.md) - Command-line interface
- [Configuration](./docs/CONFIGURATION.md) - Configuration reference

## Resources

- [Go](https://golang.org/doc/) | [Just](https://github.com/casey/just) | [Cobra](https://github.com/spf13/cobra)

> **Tip**: Docs may become outdated. When in doubt, read the code directly.

## Cursor Cloud specific instructions

Linux Cloud Agents can build and test Neru. The environment install provides Go 1.26.4, `just` 1.54.0, `golangci-lint` 2.12.2, and the CGO libraries in [Linux Setup](docs/LINUX_SETUP.md), including `libxext-dev` and `libxrender-dev` (`pkg-config` modules `xext` and `xrender`).

- Build and checks: `just build`, `just vet`, `just lint`, `just test`. Fast core slice: `just test-foundation`.
- Headless product check: `./bin/neru config init --config <path>` then `./bin/neru config validate --config <path>`.
- `./bin/neru launch` stays in the foreground and serves `/tmp/neru.sock`. On an X11 `DISPLAY` it reports `linux/x11`. Overlay modes need that session. Stop a running daemon before `just test`; integration tests use the same socket.
- `neru doctor` exits non-zero when the daemon is down. On a minimal X11 session, AT-SPI app watching, notifications, and dark-mode detection can stay stubbed while IPC, grid, scroll, hotkeys, and overlay still report healthy.
- The startup message that Linux features are stubs is behind the X11 and Wayland backends. Use `neru status` and [Linux Setup](docs/LINUX_SETUP.md).
