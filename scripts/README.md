# Installation Scripts

This directory contains robust, platform-specific installation scripts for Lil-RAG.

## Quick Install

**Universal installer (recommended):**
```bash
curl -fsSL https://raw.githubusercontent.com/streed/lil-rag/main/install.sh | bash
```

This detects your platform and downloads the appropriate installer automatically.

## Platform-Specific Installers

### Linux: `install-linux.sh`

Comprehensive Linux installer with support for multiple distributions.

**Features:**
- Auto-detects Linux distribution (Ubuntu, Debian, CentOS, RHEL, Fedora, Arch, openSUSE, Alpine)
- Automatically installs dependencies via package managers
- Installs `pdftotext` (poppler-utils) for optimal PDF extraction
- Supports multiple architectures (amd64, arm64, armv7, armv6, 386)
- Optional Go installation for building from source
- Comprehensive system requirements checking
- PATH configuration assistance

**Usage:**
```bash
# Direct download and run
curl -fsSL https://raw.githubusercontent.com/streed/lil-rag/main/scripts/install-linux.sh | bash

# Or download and run locally
wget https://raw.githubusercontent.com/streed/lil-rag/main/scripts/install-linux.sh
chmod +x install-linux.sh
./install-linux.sh
```

**Options:**
- `-d, --dir DIR`: Install directory (default: `~/.local/bin`)
- `--skip-deps`: Skip dependency installation
- `--skip-verify`: Skip installation verification
- `--force`: Force installation even if already installed

### macOS: `install-macos.sh`

Native macOS installer with Homebrew integration.

**Features:**
- Apple Silicon (M1/M2/M3) and Intel support
- Homebrew integration for dependency management
- Automatic Xcode Command Line Tools installation
- macOS security and quarantine handling
- Installs `poppler` for PDF text extraction
- Optional Go installation via Homebrew
- Shell configuration assistance (bash, zsh, fish)

**Usage:**
```bash
# Direct download and run
curl -fsSL https://raw.githubusercontent.com/streed/lil-rag/main/scripts/install-macos.sh | bash

# Or download and run locally
curl -O https://raw.githubusercontent.com/streed/lil-rag/main/scripts/install-macos.sh
chmod +x install-macos.sh
./install-macos.sh
```

**Options:**
- `-d, --dir DIR`: Install directory (default: `~/.local/bin`)
- `--skip-deps`: Skip dependency installation
- `--skip-homebrew`: Skip Homebrew installation prompt
- `--skip-verify`: Skip installation verification
- `--force`: Force installation even if already installed

### Windows: `install-windows.ps1`

PowerShell installer for Windows with Chocolatey integration.

**Features:**
- Windows 10+ support with architecture detection (AMD64, ARM64)
- Chocolatey integration for dependency management
- Installs `poppler` for PDF text extraction via Chocolatey
- Windows Defender and antivirus compatibility notes
- Optional Go installation via Chocolatey
- PATH configuration assistance
- PowerShell execution policy handling

**Usage:**
```powershell
# Direct download and run (PowerShell as Administrator recommended)
iwr -useb https://raw.githubusercontent.com/streed/lil-rag/main/scripts/install-windows.ps1 | iex

# Or download and run locally
Invoke-WebRequest -Uri https://raw.githubusercontent.com/streed/lil-rag/main/scripts/install-windows.ps1 -OutFile install-windows.ps1
.\install-windows.ps1
```

**Options:**
- `-InstallDir DIR`: Install directory (default: `%LOCALAPPDATA%\lil-rag\bin`)
- `-SkipDeps`: Skip dependency installation
- `-SkipChocolatey`: Skip Chocolatey installation prompt
- `-SkipVerify`: Skip installation verification
- `-Force`: Force installation even if already installed

## Development Usage

When working on the installation scripts locally, you can test them using:

```bash
# Test universal installer with local scripts
./install.sh --local

# Test individual platform scripts directly
./scripts/install-linux.sh
./scripts/install-macos.sh
./scripts/install-windows.ps1  # In PowerShell
```

## Features Common to All Installers

### Comprehensive System Checking
- Operating system and architecture detection
- System requirements verification
- Missing tool detection and installation
- Version compatibility checking

### Dependency Management
- **pdftotext/poppler**: Required for optimal PDF text extraction
- **Go**: Optional, for building from source
- **Ollama**: Recommended, with installation guidance
- Package manager integration (apt, yum, dnf, pacman, brew, choco)

### Robust Installation Process
- GitHub releases API integration
- Secure download with verification
- Binary extraction and installation
- Executable permissions and security handling
- Installation verification with version checking

### User Experience
- Colored output with clear progress indication
- Comprehensive error handling and reporting
- PATH configuration assistance
- Post-installation setup guidance
- Platform-specific security notes

### Security Features
- Checksum verification (where available)
- Secure download protocols (HTTPS)
- Quarantine attribute removal (macOS)
- Execution policy handling (Windows)
- Temporary file cleanup

## Architecture Support

| Platform | AMD64 | ARM64 | ARMv7 | ARMv6 | 386 |
|----------|-------|-------|-------|-------|-----|
| Linux    | ✅     | ✅     | ✅     | ✅     | ✅   |
| macOS    | ✅     | ✅     | ❌     | ❌     | ❌   |
| Windows  | ✅     | ✅     | ❌     | ❌     | ❌   |

## Troubleshooting

### Linux
- **Permission denied**: Ensure script is executable (`chmod +x`)
- **Package installation fails**: Check if running with appropriate privileges
- **Unknown distribution**: Install dependencies manually

### macOS
- **"Cannot be opened" errors**: Check System Preferences > Security or run `sudo spctl --master-disable`
- **Homebrew installation fails**: Ensure Xcode Command Line Tools are installed
- **Architecture mismatch**: The script auto-detects Apple Silicon vs Intel

### Windows
- **Script execution blocked**: Run `Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser`
- **Antivirus interference**: Add installation directory to antivirus exclusions
- **PowerShell not found**: Install PowerShell or use WSL with Linux installer

## Contributing

When modifying the installation scripts:

1. Test on the target platform
2. Ensure error handling is comprehensive
3. Update the usage documentation
4. Test both online and local (`--local`) modes
5. Verify cleanup on both success and failure paths

## License

These installation scripts are part of the Lil-RAG project and are licensed under the MIT License.