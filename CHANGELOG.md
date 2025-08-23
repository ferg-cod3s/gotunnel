# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Comprehensive documentation suite including architecture, API, and user guides
- TODO.md for tracking project tasks and priorities
- Security documentation and vulnerability reporting guidelines
- Contributing guidelines for community collaboration

### Changed
- Improved README with better organization and clearer instructions
- Enhanced error messages for better user experience

### Fixed
- Documentation structure aligned with best practices

## [0.1.0-beta] - 2024-08-13

### Added
- Initial beta release of gotunnel
- Core tunneling functionality without root privileges
- Built-in HTTP/HTTPS proxy support
- mDNS auto-discovery for network-wide access
- Multi-platform support (macOS, Linux, Windows)
- Docker containerization with compose support
- Basic CLI with start, stop, list commands
- Self-signed certificate generation for HTTPS
- Hosts file management with backup/restore
- Multiple concurrent tunnel support
- Configuration through CLI flags
- Environment variable support for configuration
- Basic logging functionality
- Support for nginx and Caddy backends
- Non-privileged port alternatives (8080/8443)
- Installation scripts for Unix systems
- Homebrew tap for macOS installation
- Scoop bucket for Windows installation
- Basic error handling and recovery

### Known Issues
- Test suite failures due to port conflicts
- Privilege detection needs improvement
- Windows platform support incomplete
- No configuration file support yet
- Limited observability and monitoring
- No CI/CD pipeline implemented
- Certificate rotation not implemented
- Connection pooling not optimized

### Security
- No root privileges required for core functionality
- Automatic certificate management
- Secure hosts file operations with backups
- Input validation for tunnel configurations

## [0.0.1-alpha] - 2024-11-01 [Internal]

### Added
- Initial project structure
- Basic tunnel proof of concept
- Command-line interface skeleton
- Initial testing framework

---

## Version History Summary

| Version | Date | Status | Key Features |
|---------|------|--------|--------------|
| 0.1.0-beta | 2024-08-13 | Current | Initial public beta |
| 0.0.1-alpha | 2024-11-01 | Internal | Initial development |

## Upgrade Notes

### Upgrading to 0.1.0-beta
- First public release, no upgrade path needed
- Fresh installation recommended
- Backup any existing tunnel configurations

## Deprecation Notices
- None at this time

## Future Releases

### [0.2.0] - Planned
- OpenTelemetry integration
- Sentry error tracking
- Configuration file support (YAML/JSON)
- Improved test coverage
- CI/CD pipeline
- Enhanced Windows support

### [0.3.0] - Planned
- Web UI for tunnel management
- Advanced routing features
- Load balancing support
- API for programmatic control

### [1.0.0] - Planned
- Production-ready release
- Complete documentation
- Full platform support
- Enterprise features

---

*For detailed migration guides, see [docs/MIGRATION.md](docs/MIGRATION.md)*
*For security updates, see [SECURITY.md](SECURITY.md)*
