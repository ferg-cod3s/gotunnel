#!/bin/bash
set -e

# gotunnel installation script
# Usage: curl -sSL https://raw.githubusercontent.com/johncferguson/gotunnel/main/scripts/install.sh | bash

REPO="johncferguson/gotunnel"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="gotunnel"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

log_success() {
    echo -e "${GREEN}✅${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}⚠️${NC} $1"
}

log_error() {
    echo -e "${RED}❌${NC} $1"
}

# Detect OS and architecture
detect_platform() {
    local os arch
    
    # Detect OS
    case "$(uname -s)" in
        Linux*)   os="linux" ;;
        Darwin*)  os="darwin" ;;
        MINGW*|CYGWIN*|MSYS*) os="windows" ;;
        *)        
            log_error "Unsupported operating system: $(uname -s)"
            exit 1
            ;;
    esac
    
    # Detect architecture
    case "$(uname -m)" in
        x86_64|amd64) arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *)
            log_error "Unsupported architecture: $(uname -m)"
            exit 1
            ;;
    esac
    
    # Set platform-specific values
    PLATFORM="${os}-${arch}"
    if [[ "$os" == "windows" ]]; then
        BINARY_NAME="gotunnel.exe"
        INSTALL_DIR="$HOME/bin"
    fi
    
    log_info "Detected platform: $PLATFORM"
}

# Check if running as root (except on Windows)
check_permissions() {
    if [[ "$PLATFORM" != "windows"* ]] && [[ $EUID -eq 0 ]] && [[ -z "$FORCE_ROOT" ]]; then
        log_warning "Running as root. Install will be system-wide."
        log_info "To install for current user only, run: INSTALL_DIR=\$HOME/.local/bin $0"
    fi
    
    # For user installation, use ~/.local/bin
    if [[ "$PLATFORM" != "windows"* ]] && [[ $EUID -ne 0 ]]; then
        INSTALL_DIR="$HOME/.local/bin"
        log_info "Installing to user directory: $INSTALL_DIR"
    fi
}

# Get the latest version from GitHub API
get_latest_version() {
    log_info "Fetching latest version..."
    
    if command -v curl >/dev/null; then
        VERSION=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/')
    elif command -v wget >/dev/null; then
        VERSION=$(wget -qO- "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/')
    else
        log_error "Neither curl nor wget found. Please install one of them."
        exit 1
    fi
    
    if [[ -z "$VERSION" ]]; then
        log_error "Failed to get latest version"
        exit 1
    fi
    
    log_info "Latest version: $VERSION"
}

# Download and install binary
install_binary() {
    local download_url="https://github.com/$REPO/releases/download/$VERSION/gotunnel-$VERSION-$PLATFORM"
    local tmp_file="/tmp/gotunnel-$VERSION-$PLATFORM"
    
    log_info "Downloading gotunnel $VERSION for $PLATFORM..."
    
    # Download binary
    if command -v curl >/dev/null; then
        curl -sL "$download_url" -o "$tmp_file"
    elif command -v wget >/dev/null; then
        wget -q "$download_url" -O "$tmp_file"
    else
        log_error "Neither curl nor wget found"
        exit 1
    fi
    
    if [[ ! -f "$tmp_file" ]]; then
        log_error "Failed to download binary"
        exit 1
    fi
    
    # Verify download (optional checksum verification)
    log_info "Verifying download..."
    if [[ -s "$tmp_file" ]]; then
        log_success "Download verified"
    else
        log_error "Downloaded file is empty or corrupt"
        exit 1
    fi
    
    # Create install directory
    mkdir -p "$INSTALL_DIR"
    
    # Install binary
    log_info "Installing to $INSTALL_DIR/$BINARY_NAME..."
    
    if [[ "$PLATFORM" == "windows"* ]]; then
        cp "$tmp_file" "$INSTALL_DIR/$BINARY_NAME"
    else
        sudo cp "$tmp_file" "$INSTALL_DIR/$BINARY_NAME" 2>/dev/null || cp "$tmp_file" "$INSTALL_DIR/$BINARY_NAME"
        chmod +x "$INSTALL_DIR/$BINARY_NAME"
    fi
    
    # Clean up
    rm -f "$tmp_file"
    
    log_success "gotunnel installed successfully!"
}

# Add to PATH if needed
setup_path() {
    if [[ "$PLATFORM" == "windows"* ]]; then
        log_info "Add $INSTALL_DIR to your PATH environment variable"
        return
    fi
    
    # Check if install directory is in PATH
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        log_warning "$INSTALL_DIR is not in your PATH"
        
        # Suggest adding to shell profile
        local shell_profile
        case "$SHELL" in
            */bash) shell_profile="$HOME/.bashrc" ;;
            */zsh)  shell_profile="$HOME/.zshrc" ;;
            */fish) shell_profile="$HOME/.config/fish/config.fish" ;;
            *)      shell_profile="$HOME/.profile" ;;
        esac
        
        log_info "Add the following to your $shell_profile:"
        echo "export PATH=\"$INSTALL_DIR:\$PATH\""
        echo
        log_info "Then reload your shell or run: source $shell_profile"
    fi
}

# Verify installation
verify_installation() {
    log_info "Verifying installation..."
    
    if command -v "$BINARY_NAME" >/dev/null; then
        local installed_version
        installed_version=$("$BINARY_NAME" --version 2>/dev/null | head -n1 || echo "unknown")
        log_success "gotunnel is installed and accessible"
        log_info "Installed version: $installed_version"
    else
        local full_path="$INSTALL_DIR/$BINARY_NAME"
        if [[ -x "$full_path" ]]; then
            local installed_version
            installed_version=$("$full_path" --version 2>/dev/null | head -n1 || echo "unknown")
            log_success "gotunnel is installed at $full_path"
            log_info "Installed version: $installed_version"
        else
            log_error "Installation verification failed"
            exit 1
        fi
    fi
}

# Show usage examples
show_usage() {
    echo
    log_success "Installation complete! Here are some examples:"
    echo
    echo "  # Start a basic tunnel (no privileges required)"
    echo "  gotunnel --proxy=builtin start --port 3000 --domain myapp"
    echo
    echo "  # Start with custom proxy ports"  
    echo "  gotunnel --proxy=builtin --proxy-http-port 8080 start --port 3000 --domain myapp"
    echo
    echo "  # Generate nginx config instead of running proxy"
    echo "  gotunnel --proxy=config start --port 3000 --domain myapp"
    echo
    echo "  # Get help"
    echo "  gotunnel --help"
    echo
    log_info "Visit https://gotunnel.dev for documentation and examples"
}

# Main installation flow
main() {
    echo "🚇 gotunnel installer"
    echo "====================="
    echo
    
    detect_platform
    check_permissions
    get_latest_version
    install_binary
    setup_path
    verify_installation
    show_usage
    
    echo
    log_success "🎉 gotunnel installation completed successfully!"
}

# Handle command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --version)
            VERSION="$2"
            shift 2
            ;;
        --install-dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        --force-root)
            FORCE_ROOT=1
            shift
            ;;
        -h|--help)
            echo "gotunnel installer"
            echo
            echo "Usage: $0 [options]"
            echo
            echo "Options:"
            echo "  --version VERSION    Install specific version"
            echo "  --install-dir DIR    Install to specific directory"
            echo "  --force-root         Allow running as root"
            echo "  -h, --help          Show this help"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Run installation
main