# Contributing to gotunnel

First off, thank you for considering contributing to gotunnel! It's people like you that make gotunnel such a great tool. We welcome contributions from everyone, regardless of experience level.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [How Can I Contribute?](#how-can-i-contribute)
- [Development Setup](#development-setup)
- [Development Guidelines](#development-guidelines)
- [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Style Guide](#style-guide)
- [Commit Messages](#commit-messages)
- [Community](#community)

## Code of Conduct

This project and everyone participating in it is governed by our Code of Conduct. By participating, you are expected to uphold this code. Please be respectful and constructive in all interactions.

### Our Standards

- Be welcoming and inclusive
- Be respectful of differing viewpoints
- Accept constructive criticism gracefully
- Focus on what's best for the community
- Show empathy towards others

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** locally
3. **Create a branch** for your changes
4. **Make your changes** following our guidelines
5. **Test your changes** thoroughly
6. **Submit a pull request**

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check existing issues to avoid duplicates. When creating a bug report, please include:

- **Clear title and description**
- **Steps to reproduce** the issue
- **Expected behavior** vs actual behavior
- **System information** (OS, Go version, etc.)
- **Relevant logs or error messages**
- **Code samples** if applicable

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion, please include:

- **Use case** - Why is this enhancement needed?
- **Proposed solution** - How should it work?
- **Alternatives considered** - What other solutions did you consider?
- **Additional context** - Any other relevant information

### Contributing Code

#### First-Time Contributors

Look for issues labeled:
- `good first issue` - Simple issues perfect for beginners
- `help wanted` - Issues where we need community help
- `documentation` - Documentation improvements

#### Areas We Need Help

- **Testing**: Improving test coverage and fixing test failures
- **Documentation**: Tutorials, examples, and guides
- **Platform Support**: Windows and Linux improvements
- **Performance**: Optimizations and benchmarks
- **Features**: New functionality from our roadmap

## Development Setup

### Prerequisites

- Go 1.22 or later
- Git
- Make (optional but recommended)
- golangci-lint for code quality
- gofmt for code formatting
- go mod for dependency management

### Setup Steps

```bash
# Fork and clone the repository
git clone https://github.com/YOUR_USERNAME/gotunnel.git
cd gotunnel

# Add upstream remote
git remote add upstream https://github.com/johncferguson/gotunnel.git

# Install dependencies
go mod download
go mod tidy

# Install development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Build the project
go build ./cmd/gotunnel

# Run tests
go test ./...
```

### IDE Setup

We recommend using VS Code or GoLand with the following extensions:
- Go language support
- EditorConfig support
- Git integration
- Markdown preview

## Development Guidelines

### Code Quality Standards

Follow these principles based on our development rules:

#### Security (Severity: Error)
- Never commit secrets or credentials
- Validate and sanitize all user inputs
- Use parameterized queries for databases
- Implement proper error handling without exposing stack traces
- Follow the principle of least privilege

#### Code Simplicity (Severity: Warning)
- Prioritize readability and maintainability
- Keep functions small (10-30 lines typically)
- Maximum nesting depth of 3 levels
- Use clear, descriptive naming
- Follow SOLID principles and DRY

#### Testing Requirements
- Write tests for all new features
- Maintain or improve code coverage
- Test both happy and error paths
- Include edge cases and boundary conditions
- Mock external dependencies appropriately

### Project Structure

```
gotunnel/
├── cmd/gotunnel/       # Main application entry point
├── internal/           # Private application code
│   ├── tunnel/        # Tunnel management
│   ├── cert/          # Certificate handling
│   ├── config/        # Configuration management
│   └── observability/ # Logging and monitoring
├── pkg/               # Public libraries (if any)
├── docs/              # Documentation
├── scripts/           # Build and installation scripts
├── configs/           # Configuration examples
└── test/              # Additional test files
```

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run with race detection
go test -race ./...

# Run specific package tests
go test ./internal/tunnel -v

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Writing Tests

- Use table-driven tests where appropriate
- Follow AAA pattern (Arrange, Act, Assert)
- Use meaningful test names that describe what's being tested
- Mock external dependencies
- Clean up resources in test teardown

Example test structure:

```go
func TestTunnelCreation(t *testing.T) {
    tests := []struct {
        name    string
        config  TunnelConfig
        wantErr bool
    }{
        {
            name:    "valid configuration",
            config:  TunnelConfig{Port: 3000, Domain: "test"},
            wantErr: false,
        },
        // Add more test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

## Pull Request Process

### Before Submitting

1. **Update documentation** for user-facing changes
2. **Add tests** for new functionality
3. **Run the test suite** locally
4. **Run linters** and fix any issues:
   ```bash
   golangci-lint run
   go fmt ./...
   go vet ./...
   ```
5. **Update CHANGELOG.md** with your changes (in Unreleased section)

### PR Guidelines

1. **Create a descriptive title** that summarizes the change
2. **Reference any related issues** using GitHub keywords (fixes #123)
3. **Describe your changes** in detail in the PR description
4. **Include screenshots** for UI changes
5. **Check all CI checks pass**
6. **Respond to review feedback** promptly

### Review Process

- PRs require at least one maintainer approval
- All CI checks must pass
- No merge conflicts
- Code coverage should not decrease significantly
- Changes must follow our style guide

## Style Guide

### Go Code Style

We follow standard Go conventions with these additions:

```go
// Package comments should be present
package tunnel

// Exported types need documentation
type Manager struct {
    // Fields should be documented if not obvious
    tunnels map[string]*Tunnel
}

// Method documentation should explain what it does
// and any important side effects or requirements
func (m *Manager) CreateTunnel(config Config) (*Tunnel, error) {
    // Early returns for validation
    if err := config.Validate(); err != nil {
        return nil, fmt.Errorf("invalid config: %w", err)
    }
    
    // Group related logic
    tunnel := &Tunnel{
        ID:     generateID(),
        Config: config,
    }
    
    // Clear error handling
    if err := tunnel.Start(); err != nil {
        return nil, fmt.Errorf("failed to start tunnel: %w", err)
    }
    
    return tunnel, nil
}
```

### Documentation Style

- Use clear, concise language
- Include code examples where helpful
- Keep README focused on users
- Put detailed technical docs in /docs
- Update documentation with code changes

## Commit Messages

We use conventional commits for clear history:

### Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code changes that neither fix bugs nor add features
- `perf`: Performance improvements
- `test`: Test additions or corrections
- `build`: Build system or dependency changes
- `ci`: CI/CD configuration changes
- `chore`: Other changes that don't modify src or test files

### Examples

```
feat(tunnel): add support for custom headers

- Allow users to specify custom HTTP headers
- Headers are validated before being applied
- Added tests for header validation

Closes #123
```

```
fix(cert): resolve certificate expiry detection

The certificate expiry check was using the wrong timezone,
causing false positives. This fix ensures UTC is used
consistently.

Fixes #456
```

## Community

### Getting Help

- **GitHub Issues**: For bugs and feature requests
- **Discussions**: For questions and general discussion
- **Documentation**: Check our docs first
- **Examples**: Look at the examples/ directory

### Maintainer Responsibilities

Maintainers are responsible for:
- Reviewing and merging PRs
- Responding to issues
- Maintaining documentation
- Release management
- Community moderation

### Recognition

We value all contributions! Contributors are:
- Listed in our README
- Mentioned in release notes
- Given credit in commit messages

## License

By contributing to gotunnel, you agree that your contributions will be licensed under the MIT License.

## Questions?

Feel free to open an issue with the `question` label or start a discussion if you have any questions about contributing.

Thank you for contributing to gotunnel! 🚇
