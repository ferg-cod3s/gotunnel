# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

### Go Development
```bash
go mod tidy                  # Clean and update dependencies
go build ./cmd/gotunnel      # Build the main binary
go run ./cmd/gotunnel        # Run the application directly
go test ./...                # Run all tests
go test -v ./internal/tunnel # Run tests for specific package with verbose output
go test -run TestSpecific    # Run specific test by name
```

### Cross-Platform Building
```bash
./build.bat                  # Windows batch script for multi-platform builds
# Creates binaries for Windows, Linux, macOS (Intel and Apple Silicon)
# Output goes to bin/ directory with versioned names
```

### Testing and Verification
```bash
go test ./... -cover         # Run tests with coverage report
go test -race ./...          # Run tests with race condition detection
go vet ./...                 # Static analysis for potential issues
```

## Architecture Overview

### Core Components

**gotunnel** is a secure local tunneling tool that creates HTTPS-enabled local domains with mDNS discovery. The architecture consists of:

1. **CLI Interface** (`cmd/gotunnel/main.go`)
   - Uses `github.com/urfave/cli/v2` for command structure
   - Commands: start, stop, list, stop-all
   - Handles privilege checking and graceful shutdown

2. **Tunnel Manager** (`internal/tunnel/`)
   - Manages multiple concurrent tunnels
   - Handles HTTP/HTTPS proxy setup
   - Modifies `/etc/hosts` for local domain resolution
   - Integrates with certificate and DNS components

3. **Certificate Management** (`internal/cert/`)
   - Platform-specific certificate handling (Unix/Windows)
   - Integrates with `mkcert` for local CA certificates
   - Manages certificate storage in `./certs` directory

4. **DNS Server** (`internal/dnsserver/`)
   - mDNS service discovery using `github.com/hashicorp/mdns`
   - Enables network-wide access to local services
   - Manages service entries and cleanup

5. **Privilege Handling** (`internal/privilege/`)
   - Cross-platform privilege checking (currently disabled)
   - Required for binding to ports 80/443 and modifying system files

6. **State Management** (`internal/state/`)
   - Tracks active tunnels and their configuration
   - Enables persistent tunnel management

### Key Dependencies
- `github.com/urfave/cli/v2` - CLI framework
- `github.com/hashicorp/mdns` - mDNS service discovery
- `github.com/grandcat/zeroconf` - Zero-configuration networking
- `gopkg.in/yaml.v3` - Configuration file parsing

### Platform-Specific Code
- Certificate handling: `cert_unix.go`, `cert_windows.go`
- Tunnel operations: `tunnel_unix.go`, `tunnel_windows.go`
- Separate implementations for Windows vs Unix-like systems

## Development Patterns

### Testing Structure
- All packages include comprehensive test files (`*_test.go`)
- Uses `github.com/stretchr/testify` for assertions
- Integration tests in `cmd/gotunnel/main_test.go`
- Test coverage spans certificate handling, tunneling, DNS, and state management

### Error Handling
- Extensive error wrapping with context using `fmt.Errorf`
- Graceful shutdown with context cancellation
- Proper resource cleanup (listeners, certificates, hosts file modifications)

### Security Considerations
- Requires elevated privileges for system modifications
- Certificate validation and secure TLS configuration
- Safe hosts file modification with backup/restore capabilities
- Network isolation through local-only certificate authorities

## Usage Notes

### Development Setup
1. Ensure Go 1.21.6+ is installed
2. Clone repository and run `go mod tidy`
3. For full functionality, requires elevated privileges (sudo/admin)
4. Uses `./certs` directory for certificate storage

### Testing Considerations
- Some tests may require network access for mDNS functionality
- Certificate tests may need filesystem permissions
- Integration tests create actual network listeners

### Build Process
- Single binary output with no external dependencies
- Cross-compilation supported for all major platforms
- Version information embedded during build process