#!/bin/bash
set -e

# gotunnel uninstall script

BINARY_NAME="gotunnel"
CONFIG_DIRS=(
    "$HOME/.gotunnel"
    "$HOME/.config/gotunnel"
    "/usr/local/etc/gotunnel"
    "/etc/gotunnel"
)
INSTALL_DIRS=(
    "/usr/local/bin"
    "$HOME/.local/bin"
    "$HOME/bin"
)

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

# Detect platform
detect_platform() {
    case "$(uname -s)" in
        MINGW*|CYGWIN*|MSYS*) 
            BINARY_NAME="gotunnel.exe"
            ;;
    esac
}

# Find installed binary
find_binary() {
    local binary_path=""
    
    # Check if in PATH
    if command -v "$BINARY_NAME" >/dev/null; then
        binary_path=$(command -v "$BINARY_NAME")
        log_info "Found $BINARY_NAME in PATH: $binary_path"
        return 0
    fi
    
    # Check common install directories
    for dir in "${INSTALL_DIRS[@]}"; do
        if [[ -f "$dir/$BINARY_NAME" ]]; then
            binary_path="$dir/$BINARY_NAME"
            log_info "Found $BINARY_NAME at: $binary_path"
            return 0
        fi
    done
    
    return 1
}

# Stop any running gotunnel processes
stop_processes() {
    log_info "Stopping any running gotunnel processes..."
    
    if pgrep -f "$BINARY_NAME" >/dev/null; then
        log_warning "Found running gotunnel processes"
        
        # Graceful shutdown first
        pkill -TERM -f "$BINARY_NAME" 2>/dev/null || true
        sleep 2
        
        # Force kill if still running
        if pgrep -f "$BINARY_NAME" >/dev/null; then
            log_warning "Force killing remaining processes"
            pkill -KILL -f "$BINARY_NAME" 2>/dev/null || true
        fi
        
        log_success "Stopped gotunnel processes"
    else
        log_info "No running gotunnel processes found"
    fi
}

# Remove binary
remove_binary() {
    local binary_path=""
    local removed=false
    
    # Remove from all possible locations
    for dir in "${INSTALL_DIRS[@]}"; do
        if [[ -f "$dir/$BINARY_NAME" ]]; then
            log_info "Removing $dir/$BINARY_NAME..."
            
            if [[ -w "$dir" ]]; then
                rm -f "$dir/$BINARY_NAME"
            else
                sudo rm -f "$dir/$BINARY_NAME" 2>/dev/null || {
                    log_error "Failed to remove $dir/$BINARY_NAME (permission denied)"
                    continue
                }
            fi
            
            if [[ ! -f "$dir/$BINARY_NAME" ]]; then
                log_success "Removed $dir/$BINARY_NAME"
                removed=true
            fi
        fi
    done
    
    if [[ "$removed" == "false" ]]; then
        log_warning "No gotunnel binary found to remove"
    fi
}

# Remove configuration files
remove_config() {
    local removed=false
    
    for dir in "${CONFIG_DIRS[@]}"; do
        if [[ -d "$dir" ]]; then
            log_info "Removing configuration directory: $dir"
            
            # Show what will be removed
            echo "Contents of $dir:"
            ls -la "$dir" 2>/dev/null || true
            echo
            
            read -p "Remove $dir? [y/N]: " -n 1 -r
            echo
            
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                if [[ -w "$(dirname "$dir")" ]]; then
                    rm -rf "$dir"
                else
                    sudo rm -rf "$dir" 2>/dev/null || {
                        log_error "Failed to remove $dir (permission denied)"
                        continue
                    }
                fi
                
                if [[ ! -d "$dir" ]]; then
                    log_success "Removed $dir"
                    removed=true
                fi
            else
                log_info "Skipped $dir"
            fi
        fi
    done
    
    if [[ "$removed" == "false" ]]; then
        log_info "No configuration directories found or none removed"
    fi
}

# Remove from systemd (if applicable)
remove_systemd_service() {
    local service_files=(
        "/etc/systemd/system/gotunnel.service"
        "/usr/lib/systemd/system/gotunnel.service"
        "$HOME/.config/systemd/user/gotunnel.service"
    )
    
    for service_file in "${service_files[@]}"; do
        if [[ -f "$service_file" ]]; then
            log_info "Found systemd service: $service_file"
            
            # Stop and disable service
            if systemctl is-active --quiet gotunnel 2>/dev/null; then
                log_info "Stopping gotunnel service..."
                sudo systemctl stop gotunnel 2>/dev/null || true
            fi
            
            if systemctl is-enabled --quiet gotunnel 2>/dev/null; then
                log_info "Disabling gotunnel service..."
                sudo systemctl disable gotunnel 2>/dev/null || true
            fi
            
            # Remove service file
            read -p "Remove systemd service file $service_file? [y/N]: " -n 1 -r
            echo
            
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                sudo rm -f "$service_file" 2>/dev/null || {
                    log_error "Failed to remove $service_file"
                    continue
                }
                
                if [[ ! -f "$service_file" ]]; then
                    log_success "Removed $service_file"
                    
                    # Reload systemd
                    sudo systemctl daemon-reload 2>/dev/null || true
                fi
            fi
        fi
    done
}

# Clean up any remaining artifacts
cleanup_artifacts() {
    log_info "Cleaning up remaining artifacts..."
    
    # Remove any gotunnel entries from hosts file (if modified)
    local hosts_file="/etc/hosts"
    if [[ -f "$hosts_file" ]] && grep -q "gotunnel" "$hosts_file" 2>/dev/null; then
        log_warning "Found gotunnel entries in $hosts_file"
        read -p "Remove gotunnel entries from hosts file? [y/N]: " -n 1 -r
        echo
        
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            # Backup hosts file first
            sudo cp "$hosts_file" "$hosts_file.backup.$(date +%Y%m%d_%H%M%S)" 2>/dev/null || true
            
            # Remove gotunnel entries
            sudo sed -i.bak '/# gotunnel/d' "$hosts_file" 2>/dev/null || {
                log_error "Failed to modify hosts file"
            }
            
            log_success "Cleaned up hosts file (backup created)"
        fi
    fi
    
    # Remove any temporary files
    rm -f /tmp/gotunnel* 2>/dev/null || true
    
    log_success "Cleanup completed"
}

# Verify uninstallation
verify_uninstall() {
    log_info "Verifying uninstallation..."
    
    if command -v "$BINARY_NAME" >/dev/null; then
        log_warning "gotunnel is still accessible via PATH"
        local remaining_path=$(command -v "$BINARY_NAME")
        log_info "Remaining binary: $remaining_path"
        return 1
    fi
    
    # Check for any remaining binaries
    local found=false
    for dir in "${INSTALL_DIRS[@]}"; do
        if [[ -f "$dir/$BINARY_NAME" ]]; then
            log_warning "Binary still exists: $dir/$BINARY_NAME"
            found=true
        fi
    done
    
    if [[ "$found" == "true" ]]; then
        return 1
    fi
    
    log_success "gotunnel has been completely removed"
    return 0
}

# Show what will be removed
show_preview() {
    echo "🗑️  gotunnel uninstaller"
    echo "========================="
    echo
    log_info "This will remove:"
    echo "  • gotunnel binary from system"
    echo "  • Configuration files (with confirmation)"
    echo "  • Systemd services (if any)"
    echo "  • Process cleanup"
    echo
    log_warning "This action cannot be undone!"
    echo
    
    read -p "Continue with uninstallation? [y/N]: " -n 1 -r
    echo
    
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Uninstallation cancelled"
        exit 0
    fi
}

# Main uninstallation flow
main() {
    detect_platform
    
    # Interactive mode by default
    if [[ "$1" != "--force" ]]; then
        show_preview
    fi
    
    log_info "Starting gotunnel uninstallation..."
    echo
    
    stop_processes
    remove_binary
    remove_config
    remove_systemd_service
    cleanup_artifacts
    
    echo
    if verify_uninstall; then
        log_success "🎉 gotunnel has been successfully uninstalled!"
    else
        log_warning "Some components may still remain. Please check manually."
        exit 1
    fi
}

# Handle command line arguments
case "${1:-}" in
    --force)
        log_info "Running in force mode (non-interactive)"
        main --force
        ;;
    -h|--help)
        echo "gotunnel uninstaller"
        echo
        echo "Usage: $0 [options]"
        echo
        echo "Options:"
        echo "  --force     Run without interactive prompts"
        echo "  -h, --help  Show this help"
        exit 0
        ;;
    "")
        main
        ;;
    *)
        log_error "Unknown option: $1"
        log_info "Use --help for usage information"
        exit 1
        ;;
esac