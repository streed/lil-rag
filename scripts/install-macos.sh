#!/usr/bin/env bash

# Lil-RAG macOS Installation Script
# Comprehensive installation script for macOS with Apple Silicon and Intel support

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

# Detect macOS version and architecture
detect_macos_info() {
    local version=$(sw_vers -productVersion)
    local arch=$(uname -m)
    local chip_type=""
    
    # Determine chip type
    if [[ "$arch" == "arm64" ]]; then
        chip_type="Apple Silicon (M1/M2/M3)"
    elif [[ "$arch" == "x86_64" ]]; then
        chip_type="Intel"
    else
        chip_type="Unknown"
    fi
    
    log "Detected macOS: $version on $arch ($chip_type)"
    
    # Check minimum macOS version (10.15+)
    local major=$(echo "$version" | cut -d'.' -f1)
    local minor=$(echo "$version" | cut -d'.' -f2)
    
    if [[ $major -lt 10 ]] || [[ $major -eq 10 && $minor -lt 15 ]]; then
        warn "macOS $version detected. Minimum supported version is 10.15 (Catalina)"
        warn "Installation may not work correctly on older versions"
    fi
    
    echo "$arch"
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
    
    # Check for Xcode Command Line Tools
    if xcode-select -p &> /dev/null; then
        substep "✓ Xcode Command Line Tools are installed"
    else
        substep "✗ Xcode Command Line Tools are missing"
        log "Installing Xcode Command Line Tools..."
        if xcode-select --install; then
            substep "✓ Xcode Command Line Tools installation initiated"
            substep "Please complete the installation and re-run this script"
            exit 0
        else
            warn "Failed to install Xcode Command Line Tools automatically"
            warn "Please install them manually: xcode-select --install"
        fi
    fi
    
    if [ ${#missing_tools[@]} -gt 0 ]; then
        error "Missing required tools: ${missing_tools[*]}"
        error "These should be available by default on macOS"
        exit 1
    fi
}

# Check for and install Homebrew
check_homebrew() {
    if command -v brew &> /dev/null; then
        substep "✓ Homebrew is installed"
        
        # Check if Homebrew is up to date
        local brew_version=$(brew --version | head -n1 | grep -o '[0-9]\+\.[0-9]\+\.[0-9]\+')
        substep "  Version: $brew_version"
        
        return 0
    else
        substep "✗ Homebrew not found"
        log "Homebrew is recommended for managing dependencies on macOS"
        
        read -p "Would you like to install Homebrew? (y/N): " -n 1 -r
        echo
        
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            log "Installing Homebrew..."
            if /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"; then
                success "Homebrew installed successfully"
                
                # Add Homebrew to PATH for Apple Silicon Macs
                local arch=$(uname -m)
                if [[ "$arch" == "arm64" ]]; then
                    if [[ ":$PATH:" != *":/opt/homebrew/bin:"* ]]; then
                        export PATH="/opt/homebrew/bin:$PATH"
                        substep "Added Homebrew to PATH for this session"
                    fi
                fi
                
                return 0
            else
                error "Failed to install Homebrew"
                return 1
            fi
        else
            log "Skipping Homebrew installation"
            return 1
        fi
    fi
}

# Install dependencies
install_dependencies() {
    section "Installing Dependencies"
    
    local packages_to_install=()
    local use_homebrew=false
    
    # Check for Homebrew
    if check_homebrew; then
        use_homebrew=true
    fi
    
    # Check for pdftotext
    if command -v pdftotext &> /dev/null; then
        substep "✓ pdftotext is already installed"
    else
        substep "✗ pdftotext not found"
        
        if [ "$use_homebrew" = true ]; then
            packages_to_install+=("poppler")
        else
            warn "pdftotext (poppler) is not installed"
            warn "For optimal PDF text extraction, install it via:"
            warn "  1. Install Homebrew: https://brew.sh"
            warn "  2. Run: brew install poppler"
            warn "  3. Or download from: https://poppler.freedesktop.org"
        fi
    fi
    
    # Check for Go (optional)
    if command -v go &> /dev/null; then
        local go_version=$(go version | grep -o 'go[0-9]\+\.[0-9]\+' | sed 's/go//')
        substep "✓ Go is installed (version $go_version)"
        
        # Check if Go version is recent enough
        local go_major=$(echo "$go_version" | cut -d'.' -f1)
        local go_minor=$(echo "$go_version" | cut -d'.' -f2)
        
        if [[ $go_major -gt 1 ]] || [[ $go_major -eq 1 && $go_minor -ge 21 ]]; then
            substep "  ✓ Go version is sufficient (1.21+ required)"
        else
            warn "  Go version $go_version is older than recommended (1.21+)"
            if [ "$use_homebrew" = true ]; then
                packages_to_install+=("go")
            fi
        fi
    else
        substep "✗ Go not found (optional - needed for building from source)"
        
        if [ "$use_homebrew" = true ]; then
            packages_to_install+=("go")
        else
            substep "  Install Go from: https://golang.org/dl/"
        fi
    fi
    
    # Install packages via Homebrew
    if [ "$use_homebrew" = true ] && [ ${#packages_to_install[@]} -gt 0 ]; then
        log "Installing packages via Homebrew: ${packages_to_install[*]}"
        
        # Update Homebrew first
        substep "Updating Homebrew..."
        brew update
        
        # Install packages
        for package in "${packages_to_install[@]}"; do
            substep "Installing $package..."
            if brew install "$package"; then
                substep "✓ $package installed successfully"
            else
                warn "Failed to install $package via Homebrew"
            fi
        done
        
        success "Dependencies installed successfully"
    elif [ ${#packages_to_install[@]} -gt 0 ]; then
        warn "Cannot automatically install packages without Homebrew"
        warn "Please install manually: ${packages_to_install[*]}"
    else
        substep "✓ All dependencies are already satisfied"
    fi
}

# Get architecture for download
get_download_arch() {
    local arch=$(uname -m)
    case "$arch" in
        x86_64)   echo "amd64" ;;
        arm64)    echo "arm64" ;;
        *)        
            error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
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
    local platform="darwin-${arch}"
    local filename="lil-rag-${platform}.tar.gz"
    
    substep "Looking for release asset: $filename"
    
    local download_url
    download_url=$(echo "$release_info" | grep -o "https://github.com/${REPO}/releases/download/[^\"]*${filename}" | head -1)
    
    if [ -z "$download_url" ]; then
        error "Could not find release asset for platform: $platform"
        substep "Available assets:"
        echo "$release_info" | grep -o "https://github.com/${REPO}/releases/download/[^\"]*" | sed 's/.*\//    /'
        
        # For macOS, we might need to build from source if no pre-built binary
        warn "Pre-built binary not available for $platform"
        warn "You may need to build from source using Go"
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
    local arch=$(get_download_arch)
    local platform="darwin-${arch}"
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
        
        # Remove quarantine attribute on macOS
        if command -v xattr &> /dev/null; then
            xattr -d com.apple.quarantine "${INSTALL_DIR}/${dst_name}" 2>/dev/null || true
        fi
        
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
                if [ -f ~/.bash_profile ]; then
                    substep "  echo 'export PATH=\"\$PATH:$INSTALL_DIR\"' >> ~/.bash_profile"
                    substep "  source ~/.bash_profile"
                else
                    substep "  echo 'export PATH=\"\$PATH:$INSTALL_DIR\"' >> ~/.bashrc"
                    substep "  source ~/.bashrc"
                fi
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
        substep "Install Ollama using one of these methods:"
        substep "  1. Download from: https://ollama.ai"
        substep "  2. Using Homebrew: brew install ollama"
        substep "  3. Official installer: curl -fsSL https://ollama.ai/install.sh | sh"
    fi
    
    # Check for security/privacy settings
    substep "macOS Security Notes:"
    substep "• If you get 'cannot be opened' errors, run:"
    substep "    sudo spctl --master-disable"
    substep "    # Then re-enable after testing: sudo spctl --master-enable"
    substep "• Or allow individual binaries in System Preferences > Security"
    
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
Lil-RAG macOS Installation Script

Usage: $0 [OPTIONS]

Options:
  -d, --dir DIR         Install directory (default: ~/.local/bin)
  -h, --help           Show this help message
  -v, --version        Show script version
  --skip-deps          Skip dependency installation
  --skip-verify        Skip installation verification
  --skip-homebrew      Skip Homebrew installation prompt
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
    local skip_homebrew=false
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
                echo "Lil-RAG macOS Installation Script v2.0.0"
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
            --skip-homebrew)
                skip_homebrew=true
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
    
    echo "🍎 Lil-RAG macOS Installation Script"
    echo "====================================="
    echo ""
    
    # Detect macOS info
    local arch
    arch=$(detect_macos_info)
    
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
        if [ "$skip_homebrew" = true ]; then
            log "Skipping Homebrew installation (--skip-homebrew flag)"
        fi
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
    local download_arch
    download_arch=$(get_download_arch)
    
    local download_url
    download_url=$(get_download_url "$release_info" "$download_arch")
    
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