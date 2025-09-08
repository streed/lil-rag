#!/usr/bin/env bash

# Lil-RAG Linux Installation Script
# Comprehensive installation script for various Linux distributions

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
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Helper functions
log() { echo -e "${BLUE}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }
success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
section() { echo -e "${PURPLE}[SECTION]${NC} $1"; }
substep() { echo -e "${CYAN}  →${NC} $1"; }

# System detection
detect_linux_distro() {
    local distro=""
    local version=""
    local id=""
    
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        distro="$NAME"
        version="$VERSION_ID"
        id="$ID"
    elif [ -f /etc/lsb-release ]; then
        . /etc/lsb-release
        distro="$DISTRIB_ID"
        version="$DISTRIB_RELEASE"
        id=$(echo "$DISTRIB_ID" | tr '[:upper:]' '[:lower:]')
    elif [ -f /etc/redhat-release ]; then
        distro=$(cat /etc/redhat-release | cut -d' ' -f1)
        id=$(echo "$distro" | tr '[:upper:]' '[:lower:]')
    elif [ -f /etc/arch-release ]; then
        distro="Arch Linux"
        id="arch"
    elif [ -f /etc/debian_version ]; then
        distro="Debian"
        version=$(cat /etc/debian_version)
        id="debian"
    else
        distro="Unknown"
        id="unknown"
    fi
    
    log "Detected Linux distribution: $distro ($id) ${version:+v$version}"
    echo "$id"
}

# Check for required system tools
check_system_requirements() {
    section "Checking System Requirements"
    
    local missing_tools=()
    local tools=("curl" "tar" "gzip")
    
    for tool in "${tools[@]}"; do
        if command -v "$tool" &> /dev/null; then
            substep "✓ $tool is installed"
        else
            substep "✗ $tool is missing"
            missing_tools+=("$tool")
        fi
    done
    
    # Check for package manager availability
    local pkg_mgr=""
    if command -v apt &> /dev/null; then
        pkg_mgr="apt"
    elif command -v yum &> /dev/null; then
        pkg_mgr="yum"
    elif command -v dnf &> /dev/null; then
        pkg_mgr="dnf"
    elif command -v pacman &> /dev/null; then
        pkg_mgr="pacman"
    elif command -v zypper &> /dev/null; then
        pkg_mgr="zypper"
    elif command -v apk &> /dev/null; then
        pkg_mgr="apk"
    fi
    
    if [ -n "$pkg_mgr" ]; then
        substep "✓ Package manager detected: $pkg_mgr"
    else
        warn "No supported package manager detected"
    fi
    
    # Install missing tools if possible
    if [ ${#missing_tools[@]} -gt 0 ] && [ -n "$pkg_mgr" ]; then
        log "Installing missing system tools: ${missing_tools[*]}"
        install_system_tools "$pkg_mgr" "${missing_tools[@]}"
    elif [ ${#missing_tools[@]} -gt 0 ]; then
        error "Missing required tools: ${missing_tools[*]}"
        error "Please install them manually and re-run this script"
        exit 1
    fi
}

# Install system tools based on package manager
install_system_tools() {
    local pkg_mgr="$1"
    shift
    local tools=("$@")
    
    case "$pkg_mgr" in
        apt)
            sudo apt-get update -qq
            sudo apt-get install -y "${tools[@]}"
            ;;
        yum)
            sudo yum install -y "${tools[@]}"
            ;;
        dnf)
            sudo dnf install -y "${tools[@]}"
            ;;
        pacman)
            sudo pacman -S --noconfirm "${tools[@]}"
            ;;
        zypper)
            sudo zypper install -y "${tools[@]}"
            ;;
        apk)
            sudo apk add "${tools[@]}"
            ;;
        *)
            error "Unsupported package manager: $pkg_mgr"
            return 1
            ;;
    esac
}

# Install dependencies including pdftotext
install_dependencies() {
    section "Installing Dependencies"
    
    local distro_id=$(detect_linux_distro)
    local packages_to_install=()
    
    # Check if pdftotext is installed
    if command -v pdftotext &> /dev/null; then
        substep "✓ pdftotext is already installed"
    else
        substep "✗ pdftotext not found - adding to installation list"
        
        case "$distro_id" in
            ubuntu|debian|linuxmint|elementary)
                packages_to_install+=("poppler-utils")
                ;;
            centos|rhel|rocky|almalinux)
                packages_to_install+=("poppler-utils")
                ;;
            fedora)
                packages_to_install+=("poppler-utils")
                ;;
            arch|manjaro|endeavouros)
                packages_to_install+=("poppler")
                ;;
            opensuse*|sles)
                packages_to_install+=("poppler-tools")
                ;;
            alpine)
                packages_to_install+=("poppler-utils")
                ;;
            *)
                warn "Unknown distribution: $distro_id"
                warn "You may need to install pdftotext manually"
                ;;
        esac
    fi
    
    # Check for Go (optional but recommended for building from source)
    if command -v go &> /dev/null; then
        local go_version=$(go version | grep -o 'go[0-9]\+\.[0-9]\+' | sed 's/go//')
        substep "✓ Go is installed (version $go_version)"
    else
        substep "✗ Go not found (optional - needed only for building from source)"
        
        case "$distro_id" in
            ubuntu|debian|linuxmint|elementary)
                packages_to_install+=("golang-go")
                ;;
            centos|rhel|rocky|almalinux)
                if command -v dnf &> /dev/null; then
                    packages_to_install+=("golang")
                else
                    # EPEL might be needed for older versions
                    substep "Note: You may need to enable EPEL repository for Go"
                fi
                ;;
            fedora)
                packages_to_install+=("golang")
                ;;
            arch|manjaro|endeavouros)
                packages_to_install+=("go")
                ;;
            opensuse*|sles)
                packages_to_install+=("go")
                ;;
            alpine)
                packages_to_install+=("go")
                ;;
        esac
    fi
    
    # Install packages if any are needed
    if [ ${#packages_to_install[@]} -gt 0 ]; then
        log "Installing packages: ${packages_to_install[*]}"
        
        case "$distro_id" in
            ubuntu|debian|linuxmint|elementary)
                sudo apt-get update -qq
                sudo apt-get install -y "${packages_to_install[@]}"
                ;;
            centos|rhel|rocky|almalinux)
                if command -v dnf &> /dev/null; then
                    sudo dnf install -y "${packages_to_install[@]}"
                else
                    sudo yum install -y "${packages_to_install[@]}"
                fi
                ;;
            fedora)
                sudo dnf install -y "${packages_to_install[@]}"
                ;;
            arch|manjaro|endeavouros)
                sudo pacman -S --noconfirm "${packages_to_install[@]}"
                ;;
            opensuse*|sles)
                sudo zypper install -y "${packages_to_install[@]}"
                ;;
            alpine)
                sudo apk add "${packages_to_install[@]}"
                ;;
            *)
                warn "Cannot automatically install packages on $distro_id"
                warn "Please install manually: ${packages_to_install[*]}"
                ;;
        esac
        
        success "Dependencies installed successfully"
    else
        substep "✓ All dependencies are already satisfied"
    fi
}

# Get architecture
detect_architecture() {
    local arch=""
    case "$(uname -m)" in
        x86_64|amd64)   arch="amd64" ;;
        aarch64|arm64)  arch="arm64" ;;
        armv7l)         arch="armv7" ;;
        armv6l)         arch="armv6" ;;
        i386|i686)      arch="386" ;;
        *)              
            error "Unsupported architecture: $(uname -m)"
            exit 1
            ;;
    esac
    echo "$arch"
}

# Get the latest release info
get_latest_release() {
    section "Fetching Latest Release Information"
    
    local release_info
    substep "Contacting GitHub API..."
    
    if ! release_info=$(curl -s --connect-timeout 10 --max-time 30 "${GITHUB_API}/repos/${REPO}/releases/latest"); then
        error "Failed to fetch release information from GitHub"
        error "Please check your internet connection and try again"
        exit 1
    fi
    
    if echo "$release_info" | grep -q "Not Found"; then
        error "Repository not found or no releases available"
        error "Please check the repository: ${GITHUB_REPO}/${REPO}"
        exit 1
    fi
    
    local version=$(echo "$release_info" | grep -o '"tag_name":"[^"]*' | cut -d'"' -f4)
    substep "✓ Latest version: $version"
    
    echo "$release_info"
}

# Extract download URL
get_download_url() {
    local release_info="$1"
    local arch="$2"
    local platform="linux-${arch}"
    local filename="lil-rag-${platform}.tar.gz"
    
    substep "Looking for release asset: $filename"
    
    local download_url
    download_url=$(echo "$release_info" | grep -o "https://github.com/${REPO}/releases/download/[^\"]*${filename}" | head -1)
    
    if [ -z "$download_url" ]; then
        error "Could not find release asset for platform: $platform"
        substep "Available assets:"
        echo "$release_info" | grep -o "https://github.com/${REPO}/releases/download/[^\"]*" | sed 's/.*\//    /'
        exit 1
    fi
    
    substep "✓ Download URL: $download_url"
    echo "$download_url"
}

# Download and extract binaries
download_and_install() {
    local download_url="$1"
    local version="$2"
    
    section "Downloading and Installing Lil-RAG $version"
    
    local temp_dir
    temp_dir=$(mktemp -d)
    local filename
    filename=$(basename "$download_url")
    
    substep "Creating temporary directory: $temp_dir"
    substep "Downloading $filename..."
    
    # Download with progress bar
    if ! curl -L --progress-bar -o "${temp_dir}/${filename}" "$download_url"; then
        error "Failed to download $download_url"
        rm -rf "$temp_dir"
        exit 1
    fi
    
    substep "✓ Download completed"
    
    # Verify download
    if [ ! -f "${temp_dir}/${filename}" ] || [ ! -s "${temp_dir}/${filename}" ]; then
        error "Downloaded file is missing or empty"
        rm -rf "$temp_dir"
        exit 1
    fi
    
    substep "Extracting binaries..."
    cd "$temp_dir"
    
    if ! tar -xzf "$filename"; then
        error "Failed to extract archive"
        rm -rf "$temp_dir"
        exit 1
    fi
    
    # Find and install binaries
    local arch=$(detect_architecture)
    local platform="linux-${arch}"
    local binaries=()
    
    for binary in lil-rag lil-rag-server lil-rag-mcp; do
        if [ -f "${binary}-${platform}" ]; then
            binaries+=("${binary}-${platform}:${binary}")
        fi
    done
    
    if [ ${#binaries[@]} -eq 0 ]; then
        error "No binaries found in the downloaded archive"
        ls -la
        rm -rf "$temp_dir"
        exit 1
    fi
    
    substep "Found ${#binaries[@]} binaries to install"
    
    # Create install directory
    if [ ! -d "$INSTALL_DIR" ]; then
        substep "Creating install directory: $INSTALL_DIR"
        mkdir -p "$INSTALL_DIR"
    fi
    
    # Install binaries
    for binary_pair in "${binaries[@]}"; do
        local src_name="${binary_pair%:*}"
        local dst_name="${binary_pair#*:}"
        
        substep "Installing $dst_name..."
        
        if ! cp "$src_name" "${INSTALL_DIR}/${dst_name}"; then
            error "Failed to copy $src_name to ${INSTALL_DIR}/${dst_name}"
            rm -rf "$temp_dir"
            exit 1
        fi
        
        chmod +x "${INSTALL_DIR}/${dst_name}"
        substep "✓ $dst_name installed successfully"
    done
    
    # Cleanup
    rm -rf "$temp_dir"
    success "Installation completed successfully"
}

# Verify installation
verify_installation() {
    section "Verifying Installation"
    
    local binaries=("lil-rag" "lil-rag-server" "lil-rag-mcp")
    local all_ok=true
    
    for binary in "${binaries[@]}"; do
        local binary_path="${INSTALL_DIR}/${binary}"
        
        if [ -f "$binary_path" ] && [ -x "$binary_path" ]; then
            # Test version command
            if version_output=$("$binary_path" --version 2>/dev/null); then
                substep "✓ $binary: $version_output"
            else
                substep "✓ $binary: installed (version check unavailable)"
            fi
        else
            substep "✗ $binary: not found or not executable"
            all_ok=false
        fi
    done
    
    if [ "$all_ok" = true ]; then
        success "All binaries installed and working correctly"
    else
        error "Some binaries failed verification"
        return 1
    fi
}

# Check PATH configuration
check_path_configuration() {
    section "Checking PATH Configuration"
    
    if [[ ":$PATH:" == *":$INSTALL_DIR:"* ]]; then
        substep "✓ $INSTALL_DIR is already in PATH"
    else
        warn "$INSTALL_DIR is not in your PATH"
        substep "To add it to your PATH, run one of the following:"
        
        # Detect shell and provide appropriate instructions
        local shell_name=$(basename "$SHELL")
        case "$shell_name" in
            bash)
                substep "  echo 'export PATH=\"\$PATH:$INSTALL_DIR\"' >> ~/.bashrc"
                substep "  source ~/.bashrc"
                ;;
            zsh)
                substep "  echo 'export PATH=\"\$PATH:$INSTALL_DIR\"' >> ~/.zshrc"
                substep "  source ~/.zshrc"
                ;;
            fish)
                substep "  fish_add_path $INSTALL_DIR"
                ;;
            *)
                substep "  export PATH=\"\$PATH:$INSTALL_DIR\""
                substep "  # Add the above line to your shell's configuration file"
                ;;
        esac
        
        substep "Or run commands with full path: $INSTALL_DIR/lil-rag"
    fi
}

# Post-installation setup
post_installation_setup() {
    section "Post-Installation Setup"
    
    substep "Checking for Ollama..."
    if command -v ollama &> /dev/null; then
        substep "✓ Ollama is installed"
        
        # Check if Ollama service is running
        if pgrep -x ollama > /dev/null; then
            substep "✓ Ollama service is running"
        else
            substep "⚠ Ollama service is not running"
            substep "  Start it with: ollama serve"
        fi
    else
        warn "Ollama is not installed"
        substep "Install Ollama from: https://ollama.ai"
        substep "Or using the official installer:"
        substep "  curl -fsSL https://ollama.ai/install.sh | sh"
    fi
    
    substep "Configuration and Data Locations:"
    substep "• Configuration: ~/.config/lil-rag/config.json"
    substep "• Data storage: ~/.local/share/lil-rag/"
    substep "• Migration from ~/.lilrag/ is automatic on first run"
    substep ""
    substep "Next steps:"
    substep "1. Ensure Ollama is running: ollama serve"
    substep "2. Pull an embedding model: ollama pull nomic-embed-text"
    substep "3. Initialize lil-rag configuration: lil-rag config init"
    substep "4. Index some content: lil-rag index doc1 'Your content here'"
    substep "5. Search content: lil-rag search 'query'"
    substep "6. Start web server: lil-rag-server"
}

# Usage information
show_usage() {
    cat << EOF
Lil-RAG Linux Installation Script

Usage: $0 [OPTIONS]

Options:
  -d, --dir DIR         Install directory (default: ~/.local/bin)
  -h, --help           Show this help message
  -v, --version        Show script version
  --skip-deps          Skip dependency installation
  --skip-verify        Skip installation verification
  --force              Force installation even if already installed

Environment Variables:
  INSTALL_DIR          Override default install directory
  NO_COLOR            Disable colored output

Examples:
  $0                           # Install to ~/.local/bin
  $0 -d /usr/local/bin        # Install to /usr/local/bin  
  $0 --skip-deps              # Skip dependency installation
  INSTALL_DIR=/opt/bin $0     # Install using environment variable

EOF
}

# Main installation function
main() {
    local skip_deps=false
    local skip_verify=false
    local force_install=false
    
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
                echo "Lil-RAG Linux Installation Script v2.0.0"
                exit 0
                ;;
            --skip-deps)
                skip_deps=true
                shift
                ;;
            --skip-verify)
                skip_verify=true
                shift
                ;;
            --force)
                force_install=true
                shift
                ;;
            *)
                error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
    
    # Disable colors if requested
    if [ -n "$NO_COLOR" ]; then
        RED='' GREEN='' BLUE='' YELLOW='' PURPLE='' CYAN='' NC=''
    fi
    
    echo "🐧 Lil-RAG Linux Installation Script"
    echo "====================================="
    echo ""
    
    # Check if already installed
    if [ "$force_install" = false ] && command -v lil-rag &> /dev/null; then
        local installed_version
        installed_version=$(lil-rag --version 2>/dev/null || echo "unknown")
        warn "lil-rag is already installed: $installed_version"
        warn "Use --force to reinstall"
        exit 0
    fi
    
    # System checks
    check_system_requirements
    
    # Install dependencies
    if [ "$skip_deps" = false ]; then
        install_dependencies
    else
        log "Skipping dependency installation"
    fi
    
    # Get release information
    local release_info
    release_info=$(get_latest_release)
    
    local version
    version=$(echo "$release_info" | grep -o '"tag_name":"[^"]*' | cut -d'"' -f4)
    
    # Get download URL
    local arch
    arch=$(detect_architecture)
    
    local download_url
    download_url=$(get_download_url "$release_info" "$arch")
    
    # Download and install
    download_and_install "$download_url" "$version"
    
    # Verify installation
    if [ "$skip_verify" = false ]; then
        verify_installation
    else
        log "Skipping installation verification"
    fi
    
    # Check PATH
    check_path_configuration
    
    # Post-installation setup
    post_installation_setup
    
    echo ""
    success "🎉 Lil-RAG installation completed successfully!"
    success "Version $version is now installed in $INSTALL_DIR"
    echo ""
    
    # Final instructions
    section "Quick Start Guide"
    substep "Visit the documentation: ${GITHUB_REPO}/${REPO}#readme"
    substep "Join the community for support and updates"
}

# Run main function with all arguments
main "$@"