#!/usr/bin/env bash

# Lil-RAG Universal Installation Script
# Platform-aware dispatcher that downloads and runs the appropriate installer

set -e

# Configuration
REPO="streed/lil-rag"
SCRIPTS_BASE_URL="https://raw.githubusercontent.com/${REPO}/main/scripts"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

# Helper functions
log() { echo -e "${BLUE}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }
success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
section() { echo -e "${PURPLE}[SECTION]${NC} $1"; }

# Show usage information
show_usage() {
    cat << 'EOF'
Lil-RAG Universal Installation Script

This script detects your platform and downloads the appropriate installer.

Usage: ./install.sh [OPTIONS]

Options:
  -d, --dir DIR         Install directory (passed to platform installer)
  -h, --help           Show this help message
  -v, --version        Show script version
  --skip-deps          Skip dependency installation
  --skip-verify        Skip installation verification
  --force              Force installation even if already installed
  --local              Use local platform scripts (for development)

Environment Variables:
  INSTALL_DIR          Override default install directory
  NO_COLOR            Disable colored output

Examples:
  ./install.sh                           # Auto-detect platform and install
  ./install.sh -d /usr/local/bin        # Install to custom directory
  ./install.sh --skip-deps              # Skip dependency installation

Platform-specific installers are also available:
  - Linux:   ./scripts/install-linux.sh
  - macOS:   ./scripts/install-macos.sh
  - Windows: ./scripts/install-windows.ps1 (run with PowerShell)

EOF
}

# Detect platform
detect_platform() {
    local os=""
    
    case "$(uname -s)" in
        Darwin*)
            os="macos"
            ;;
        Linux*)
            os="linux"
            ;;
        CYGWIN*|MINGW*|MSYS*)
            os="windows"
            ;;
        *)
            error "Unsupported operating system: $(uname -s)"
            error "Supported platforms: Linux, macOS, Windows (Git Bash/WSL)"
            exit 1
            ;;
    esac
    
    echo "$os"
}

# Check for required tools
check_requirements() {
    local missing_tools=()
    
    # Check for curl
    if ! command -v curl &> /dev/null; then
        missing_tools+=("curl")
    fi
    
    # Check for basic Unix tools
    for tool in "bash" "mktemp" "chmod"; do
        if ! command -v "$tool" &> /dev/null; then
            missing_tools+=("$tool")
        fi
    done
    
    if [ ${#missing_tools[@]} -gt 0 ]; then
        error "Missing required tools: ${missing_tools[*]}"
        error "Please install them and try again"
        exit 1
    fi
}

# Download platform-specific installer
download_installer() {
    local platform="$1"
    local use_local="$2"
    local script_name=""
    local temp_dir=""
    
    case "$platform" in
        linux)
            script_name="install-linux.sh"
            ;;
        macos)
            script_name="install-macos.sh"
            ;;
        windows)
            script_name="install-windows.ps1"
            ;;
        *)
            error "Unknown platform: $platform"
            exit 1
            ;;
    esac
    
    if [ "$use_local" = true ]; then
        # Use local script (for development)
        local local_script="scripts/$script_name"
        
        if [ -f "$local_script" ]; then
            log "Using local platform installer: $local_script"
            echo "$local_script"
            return 0
        else
            error "Local script not found: $local_script"
            error "Run from the repository root or use online installer"
            exit 1
        fi
    else
        # Download from GitHub
        temp_dir=$(mktemp -d)
        local temp_script="$temp_dir/$script_name"
        local download_url="$SCRIPTS_BASE_URL/$script_name"
        
        log "Downloading $platform installer from GitHub..."
        log "URL: $download_url"
        
        if ! curl -fsSL "$download_url" -o "$temp_script"; then
            error "Failed to download platform installer"
            error "URL: $download_url"
            rm -rf "$temp_dir"
            exit 1
        fi
        
        # Make executable
        chmod +x "$temp_script"
        
        echo "$temp_script"
    fi
}

# Run platform-specific installer
run_installer() {
    local installer_script="$1"
    local platform="$2"
    shift 2
    local args=("$@")
    
    log "Running $platform installer..."
    
    case "$platform" in
        linux|macos)
            exec "$installer_script" "${args[@]}"
            ;;
        windows)
            if command -v powershell &> /dev/null; then
                # Convert bash arguments to PowerShell format
                local ps_args=()
                local i=0
                while [ $i -lt ${#args[@]} ]; do
                    case "${args[i]}" in
                        -d|--dir)
                            ps_args+=("-InstallDir")
                            ps_args+=("${args[i+1]}")
                            i=$((i + 1))
                            ;;
                        --skip-deps)
                            ps_args+=("-SkipDeps")
                            ;;
                        --skip-verify)
                            ps_args+=("-SkipVerify")
                            ;;
                        --force)
                            ps_args+=("-Force")
                            ;;
                        -h|--help)
                            ps_args+=("-Help")
                            ;;
                        -v|--version)
                            ps_args+=("-Version")
                            ;;
                        *)
                            warn "Ignoring unknown argument for Windows: ${args[i]}"
                            ;;
                    esac
                    i=$((i + 1))
                done
                
                exec powershell -ExecutionPolicy Bypass -File "$installer_script" "${ps_args[@]}"
            else
                error "PowerShell not found. Windows installation requires PowerShell."
                error "Please install PowerShell or use WSL with the Linux installer."
                exit 1
            fi
            ;;
        *)
            error "Unknown platform: $platform"
            exit 1
            ;;
    esac
}

# Cleanup function
cleanup() {
    if [ -n "$TEMP_DIR" ] && [ -d "$TEMP_DIR" ]; then
        rm -rf "$TEMP_DIR"
    fi
}

# Set trap for cleanup
trap cleanup EXIT

# Main function
main() {
    local use_local=false
    local platform_args=()
    
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_usage
                exit 0
                ;;
            -v|--version)
                echo "Lil-RAG Universal Installation Script v2.0.0"
                exit 0
                ;;
            --local)
                use_local=true
                shift
                ;;
            *)
                # Pass through all other arguments to platform installer
                platform_args+=("$1")
                shift
                ;;
        esac
    done
    
    # Disable colors if requested
    if [ -n "$NO_COLOR" ]; then
        RED='' GREEN='' BLUE='' YELLOW='' PURPLE='' NC=''
    fi
    
    echo "🚀 Lil-RAG Universal Installation Script"
    echo "========================================="
    echo ""
    
    # Check requirements
    check_requirements
    
    # Detect platform
    section "Platform Detection"
    local platform
    platform=$(detect_platform)
    
    case "$platform" in
        linux)
            log "Detected: Linux"
            log "Using: Robust Linux installer with package manager support"
            ;;
        macos)
            log "Detected: macOS"
            log "Using: macOS installer with Homebrew integration"
            ;;
        windows)
            log "Detected: Windows (Git Bash/WSL/MSYS2)"
            log "Using: PowerShell installer for Windows"
            ;;
    esac
    
    # Download appropriate installer
    section "Downloading Platform Installer"
    local installer_script
    installer_script=$(download_installer "$platform" "$use_local")
    
    if [ "$use_local" != true ]; then
        # Store temp dir for cleanup
        TEMP_DIR=$(dirname "$installer_script")
    fi
    
    success "Platform installer ready: $(basename "$installer_script")"
    
    # Run platform-specific installer
    echo ""
    section "Starting Platform-Specific Installation"
    echo ""
    
    # Run the installer (this will exec, so no code after this runs)
    run_installer "$installer_script" "$platform" "${platform_args[@]}"
}

# Ensure we're not being sourced
if [[ "${BASH_SOURCE[0]}" != "${0}" ]]; then
    error "This script should be executed, not sourced"
    exit 1
fi

# Run main function
main "$@"