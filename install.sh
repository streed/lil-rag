#!/usr/bin/env bash

# Lil-RAG Installation Script
# Downloads and installs the latest release from GitHub

set -e

# Configuration
REPO="streed/lil-rag"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
GITHUB_API="https://api.github.com"
GITHUB_REPO="https://github.com"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
log() { echo -e "${BLUE}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }
success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }

# Detect OS and architecture
detect_platform() {
    local os arch
    
    case "$(uname -s)" in
        Darwin*)    os="darwin" ;;
        Linux*)     os="linux" ;;
        CYGWIN*|MINGW*|MSYS*) os="windows" ;;
        *)          error "Unsupported operating system: $(uname -s)"; exit 1 ;;
    esac
    
    case "$(uname -m)" in
        x86_64|amd64)   arch="amd64" ;;
        aarch64|arm64)  arch="arm64" ;;
        *)              error "Unsupported architecture: $(uname -m)"; exit 1 ;;
    esac
    
    echo "${os}-${arch}"
}

# Get the latest release info from GitHub API
get_latest_release() {
    log "Fetching latest release information..."
    
    if ! command -v curl &> /dev/null; then
        error "curl is required but not installed. Please install curl and try again."
        exit 1
    fi
    
    local release_info
    release_info=$(curl -s "${GITHUB_API}/repos/${REPO}/releases/latest")
    
    if echo "$release_info" | grep -q "Not Found"; then
        error "Repository not found or no releases available"
        error "Please check the repository: ${GITHUB_REPO}/${REPO}"
        exit 1
    fi
    
    echo "$release_info"
}

# Extract download URL for the current platform
get_download_url() {
    local release_info="$1"
    local platform="$2"
    local extension=""
    
    # Determine file extension
    if [[ "$platform" == *"windows"* ]]; then
        extension=".zip"
    else
        extension=".tar.gz"
    fi
    
    local filename="lil-rag-${platform}${extension}"
    local download_url
    
    # Extract download URL using basic text processing
    download_url=$(echo "$release_info" | grep -o "https://github.com/${REPO}/releases/download/[^\"]*${filename}" | head -1)
    
    if [ -z "$download_url" ]; then
        error "Could not find release asset for platform: $platform"
        error "Available assets:"
        echo "$release_info" | grep -o "https://github.com/${REPO}/releases/download/[^\"]*" | sed 's/.*\//  - /'
        exit 1
    fi
    
    echo "$download_url"
}

# Extract version from release info
get_version() {
    local release_info="$1"
    echo "$release_info" | grep -o '"tag_name":"[^"]*' | cut -d'"' -f4
}

# Download and extract the release
download_and_extract() {
    local download_url="$1"
    local platform="$2"
    local version="$3"
    
    local temp_dir
    temp_dir=$(mktemp -d)
    local filename
    filename=$(basename "$download_url")
    
    log "Downloading lil-rag $version for $platform..."
    
    if ! curl -L -o "${temp_dir}/${filename}" "$download_url"; then
        error "Failed to download $download_url"
        rm -rf "$temp_dir"
        exit 1
    fi
    
    log "Extracting binaries..."
    
    cd "$temp_dir"
    
    if [[ "$filename" == *.zip ]]; then
        if ! command -v unzip &> /dev/null; then
            error "unzip is required but not installed. Please install unzip and try again."
            rm -rf "$temp_dir"
            exit 1
        fi
        unzip -q "$filename"
    else
        tar -xzf "$filename"
    fi
    
    # Find the extracted binaries
    local binaries=()
    for binary in lil-rag lil-rag-server lil-rag-mcp; do
        if [[ "$platform" == *"windows"* ]]; then
            if [ -f "${binary}-${platform}.exe" ]; then
                binaries+=("${binary}-${platform}.exe:${binary}.exe")
            fi
        else
            if [ -f "${binary}-${platform}" ]; then
                binaries+=("${binary}-${platform}:${binary}")
            fi
        fi
    done
    
    if [ ${#binaries[@]} -eq 0 ]; then
        error "No binaries found in the downloaded archive"
        rm -rf "$temp_dir"
        exit 1
    fi
    
    # Create install directory if it doesn't exist
    mkdir -p "$INSTALL_DIR"
    
    log "Installing binaries to $INSTALL_DIR..."
    
    for binary_pair in "${binaries[@]}"; do
        local src_name="${binary_pair%:*}"
        local dst_name="${binary_pair#*:}"
        
        cp "$src_name" "${INSTALL_DIR}/${dst_name}"
        chmod +x "${INSTALL_DIR}/${dst_name}"
        success "Installed ${dst_name}"
    done
    
    # Cleanup
    rm -rf "$temp_dir"
}

# Check if install directory is in PATH
check_path() {
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        warn "Install directory $INSTALL_DIR is not in your PATH"
        warn "Add the following line to your shell profile (.bashrc, .zshrc, etc.):"
        echo ""
        echo "    export PATH=\"\$PATH:$INSTALL_DIR\""
        echo ""
        warn "Or run: echo 'export PATH=\"\$PATH:$INSTALL_DIR\"' >> ~/.bashrc"
    fi
}

# Verify installation
verify_installation() {
    log "Verifying installation..."
    
    local binaries=("lil-rag" "lil-rag-server")
    if [[ "$platform" == "windows" ]]; then
        binaries=("lil-rag.exe" "lil-rag-server.exe")
    fi
    
    for binary in "${binaries[@]}"; do
        if [ -f "${INSTALL_DIR}/${binary}" ]; then
            local version
            if version=$("${INSTALL_DIR}/${binary}" --version 2>/dev/null); then
                success "${binary} installed successfully: $version"
            else
                success "${binary} installed (version check failed)"
            fi
        else
            error "Binary not found: ${INSTALL_DIR}/${binary}"
        fi
    done
}

# Show usage information
show_usage() {
    echo "Lil-RAG Installation Script"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -d, --dir DIR     Install directory (default: ~/.local/bin)"
    echo "  -h, --help        Show this help message"
    echo "  -v, --version     Show version and exit"
    echo ""
    echo "Environment Variables:"
    echo "  INSTALL_DIR       Override default install directory"
    echo ""
    echo "Examples:"
    echo "  $0                          # Install to ~/.local/bin"
    echo "  $0 -d /usr/local/bin        # Install to /usr/local/bin"
    echo "  INSTALL_DIR=/opt/bin $0     # Install using environment variable"
}

# Main installation function
main() {
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -d|--dir)
                INSTALL_DIR="$2"
                shift 2
                ;;
            -h|--help)
                show_usage
                exit 0
                ;;
            -v|--version)
                echo "Lil-RAG Installation Script v1.0.0"
                exit 0
                ;;
            *)
                error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
    
    echo "🤖 Lil-RAG Installation Script"
    echo "=============================="
    echo ""
    
    # Detect platform
    local platform
    platform=$(detect_platform)
    log "Detected platform: $platform"
    
    # Get latest release
    local release_info
    release_info=$(get_latest_release)
    
    local version
    version=$(get_version "$release_info")
    log "Latest version: $version"
    
    # Get download URL
    local download_url
    download_url=$(get_download_url "$release_info" "$platform")
    log "Download URL: $download_url"
    
    # Download and install
    download_and_extract "$download_url" "$platform" "$version"
    
    # Verify installation
    verify_installation
    
    # Check PATH
    check_path
    
    echo ""
    success "Installation completed successfully!"
    success "You can now use: lil-rag, lil-rag-server"
    echo ""
    log "Quick start:"
    echo "  1. Initialize configuration: lil-rag config init"
    echo "  2. Index some content: lil-rag index doc1 'Your text content'"
    echo "  3. Search: lil-rag search 'query'"
    echo "  4. Start server: lil-rag-server"
    echo ""
    log "For more information, visit: ${GITHUB_REPO}/${REPO}"
}

# Run main function
main "$@"
