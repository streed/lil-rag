# Lil-RAG Windows Installation Script
# Comprehensive PowerShell installation script for Windows with architecture detection

param(
    [Parameter(Position = 0)]
    [string]$InstallDir = "$env:LOCALAPPDATA\lil-rag\bin",
    
    [switch]$SkipDeps,
    [switch]$SkipVerify,
    [switch]$SkipChocolatey,
    [switch]$Force,
    [switch]$Help,
    [switch]$Version
)

# Configuration
$script:REPO = "streed/lil-rag"
$script:GITHUB_API = "https://api.github.com"
$script:GITHUB_REPO = "https://github.com"

# Color codes for Windows
$script:Colors = @{
    Red    = "Red"
    Green  = "Green"
    Blue   = "Blue"
    Yellow = "Yellow"
    Purple = "Magenta"
    Cyan   = "Cyan"
    White  = "White"
}

# Helper functions for output
function Write-Log {
    param([string]$Message)
    Write-Host "[INFO] $Message" -ForegroundColor $script:Colors.Blue
}

function Write-Warning {
    param([string]$Message)
    Write-Host "[WARN] $Message" -ForegroundColor $script:Colors.Yellow
}

function Write-Error {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor $script:Colors.Red
}

function Write-Success {
    param([string]$Message)
    Write-Host "[SUCCESS] $Message" -ForegroundColor $script:Colors.Green
}

function Write-Section {
    param([string]$Message)
    Write-Host "[SECTION] $Message" -ForegroundColor $script:Colors.Purple
}

function Write-SubStep {
    param([string]$Message)
    Write-Host "  → $Message" -ForegroundColor $script:Colors.Cyan
}

# Show usage information
function Show-Usage {
    @"
Lil-RAG Windows Installation Script

Usage: .\install-windows.ps1 [OPTIONS]

Options:
  -InstallDir DIR      Install directory (default: %LOCALAPPDATA%\lil-rag\bin)
  -Help               Show this help message
  -Version            Show script version
  -SkipDeps           Skip dependency installation
  -SkipVerify         Skip installation verification
  -SkipChocolatey     Skip Chocolatey installation prompt
  -Force              Force installation even if already installed

Environment Variables:
  INSTALL_DIR         Override default install directory

Examples:
  .\install-windows.ps1                              # Install to default location
  .\install-windows.ps1 -InstallDir "C:\tools\bin"  # Install to custom directory
  .\install-windows.ps1 -SkipDeps                   # Skip dependency installation

"@
}

# Detect Windows version and architecture
function Get-WindowsInfo {
    Write-Section "Detecting System Information"
    
    $osInfo = Get-CimInstance -ClassName Win32_OperatingSystem
    $procInfo = Get-CimInstance -ClassName Win32_Processor
    
    $osVersion = "$($osInfo.Caption) $($osInfo.Version)"
    $architecture = $env:PROCESSOR_ARCHITECTURE
    
    # Map architecture names
    $archMap = @{
        "AMD64" = "amd64"
        "x86"   = "386"
        "ARM64" = "arm64"
    }
    
    $downloadArch = $archMap[$architecture]
    if (-not $downloadArch) {
        Write-Error "Unsupported architecture: $architecture"
        exit 1
    }
    
    Write-Log "Operating System: $osVersion"
    Write-Log "Architecture: $architecture ($downloadArch)"
    Write-Log "PowerShell Version: $($PSVersionTable.PSVersion)"
    
    # Check minimum Windows version (Windows 10 recommended)
    $buildNumber = [int]$osInfo.BuildNumber
    if ($buildNumber -lt 10240) {  # Windows 10 Build 10240
        Write-Warning "Windows build $buildNumber detected. Windows 10 or later is recommended."
        Write-Warning "Installation may not work correctly on older versions."
    }
    
    return $downloadArch
}

# Check for required system tools
function Test-SystemRequirements {
    Write-Section "Checking System Requirements"
    
    $missingTools = @()
    
    # Check PowerShell version
    if ($PSVersionTable.PSVersion.Major -lt 5) {
        Write-SubStep "✗ PowerShell version $($PSVersionTable.PSVersion) is too old"
        $missingTools += "PowerShell 5.0+"
    } else {
        Write-SubStep "✓ PowerShell version $($PSVersionTable.PSVersion) is sufficient"
    }
    
    # Check for .NET Framework
    try {
        $netVersion = Get-ItemProperty "HKLM:SOFTWARE\Microsoft\NET Framework Setup\NDP\v4\Full\" -Name Release -ErrorAction SilentlyContinue
        if ($netVersion -and $netVersion.Release -ge 461808) {  # .NET 4.7.2
            Write-SubStep "✓ .NET Framework 4.7.2+ is installed"
        } else {
            Write-SubStep "✗ .NET Framework 4.7.2+ is required"
            $missingTools += ".NET Framework 4.7.2+"
        }
    } catch {
        Write-SubStep "⚠ Could not verify .NET Framework version"
    }
    
    # Check for Windows Defender or Antivirus interference
    try {
        $defender = Get-MpPreference -ErrorAction SilentlyContinue
        if ($defender) {
            Write-SubStep "⚠ Windows Defender is active - may interfere with downloads"
            Write-SubStep "  If installation fails, temporarily disable real-time protection"
        }
    } catch {
        # Defender cmdlets not available, ignore
    }
    
    if ($missingTools.Count -gt 0) {
        Write-Error "Missing system requirements: $($missingTools -join ', ')"
        Write-Error "Please install them and re-run this script"
        exit 1
    }
    
    Write-Success "System requirements satisfied"
}

# Check for and install Chocolatey
function Test-Chocolatey {
    if (Get-Command choco -ErrorAction SilentlyContinue) {
        Write-SubStep "✓ Chocolatey is installed"
        
        # Check Chocolatey version
        try {
            $chocoVersion = & choco --version
            Write-SubStep "  Version: $chocoVersion"
        } catch {
            Write-SubStep "  Version check failed"
        }
        
        return $true
    } else {
        Write-SubStep "✗ Chocolatey not found"
        Write-Log "Chocolatey is recommended for managing dependencies on Windows"
        
        if (-not $SkipChocolatey) {
            $response = Read-Host "Would you like to install Chocolatey? (y/N)"
            
            if ($response -match '^[Yy]') {
                Write-Log "Installing Chocolatey..."
                
                # Check if running as Administrator
                $isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")
                
                if (-not $isAdmin) {
                    Write-Error "Administrator privileges required to install Chocolatey"
                    Write-Error "Please run PowerShell as Administrator and try again"
                    return $false
                }
                
                try {
                    Set-ExecutionPolicy Bypass -Scope Process -Force
                    [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
                    iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))
                    
                    # Refresh environment variables
                    $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")
                    
                    Write-Success "Chocolatey installed successfully"
                    return $true
                } catch {
                    Write-Error "Failed to install Chocolatey: $($_.Exception.Message)"
                    return $false
                }
            } else {
                Write-Log "Skipping Chocolatey installation"
                return $false
            }
        } else {
            Write-Log "Skipping Chocolatey installation (--skip-chocolatey flag)"
            return $false
        }
    }
}

# Install dependencies
function Install-Dependencies {
    Write-Section "Installing Dependencies"
    
    $packagesToInstall = @()
    $useChocolatey = Test-Chocolatey
    
    # Check for pdftotext (poppler)
    $pdftotext = Get-Command pdftotext -ErrorAction SilentlyContinue
    if ($pdftotext) {
        Write-SubStep "✓ pdftotext is already installed"
    } else {
        Write-SubStep "✗ pdftotext not found"
        
        if ($useChocolatey) {
            $packagesToInstall += "poppler"
        } else {
            Write-Warning "pdftotext (poppler) is not installed"
            Write-Warning "For optimal PDF text extraction, install it via:"
            Write-Warning "  1. Install Chocolatey: https://chocolatey.org/install"
            Write-Warning "  2. Run: choco install poppler"
            Write-Warning "  3. Or download from: https://github.com/oschwartz10612/poppler-windows/releases"
        }
    }
    
    # Check for Git (optional)
    $git = Get-Command git -ErrorAction SilentlyContinue
    if ($git) {
        $gitVersion = & git --version
        Write-SubStep "✓ Git is installed ($gitVersion)"
    } else {
        Write-SubStep "✗ Git not found (optional - useful for development)"
        
        if ($useChocolatey) {
            $packagesToInstall += "git"
        } else {
            Write-SubStep "  Install Git from: https://git-scm.com/download/win"
        }
    }
    
    # Check for Go (optional)
    $go = Get-Command go -ErrorAction SilentlyContinue
    if ($go) {
        $goVersion = & go version
        Write-SubStep "✓ Go is installed ($goVersion)"
        
        # Check Go version
        if ($goVersion -match 'go(\d+)\.(\d+)') {
            $goMajor = [int]$matches[1]
            $goMinor = [int]$matches[2]
            
            if ($goMajor -gt 1 -or ($goMajor -eq 1 -and $goMinor -ge 21)) {
                Write-SubStep "  ✓ Go version is sufficient (1.21+ required)"
            } else {
                Write-Warning "  Go version is older than recommended (1.21+)"
                if ($useChocolatey) {
                    $packagesToInstall += "golang"
                }
            }
        }
    } else {
        Write-SubStep "✗ Go not found (optional - needed for building from source)"
        
        if ($useChocolatey) {
            $packagesToInstall += "golang"
        } else {
            Write-SubStep "  Install Go from: https://golang.org/dl/"
        }
    }
    
    # Install packages via Chocolatey
    if ($useChocolatey -and $packagesToInstall.Count -gt 0) {
        Write-Log "Installing packages via Chocolatey: $($packagesToInstall -join ', ')"
        
        foreach ($package in $packagesToInstall) {
            Write-SubStep "Installing $package..."
            try {
                & choco install $package -y --no-progress
                Write-SubStep "✓ $package installed successfully"
            } catch {
                Write-Warning "Failed to install $package via Chocolatey: $($_.Exception.Message)"
            }
        }
        
        # Refresh PATH
        $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")
        
        Write-Success "Dependencies installed successfully"
    } elseif ($packagesToInstall.Count -gt 0) {
        Write-Warning "Cannot automatically install packages without Chocolatey"
        Write-Warning "Please install manually: $($packagesToInstall -join ', ')"
    } else {
        Write-SubStep "✓ All dependencies are already satisfied"
    }
}

# Get the latest release information
function Get-LatestRelease {
    Write-Section "Fetching Latest Release Information"
    
    Write-SubStep "Contacting GitHub API..."
    
    try {
        $releaseInfo = Invoke-RestMethod -Uri "$script:GITHUB_API/repos/$script:REPO/releases/latest" -TimeoutSec 30
        Write-SubStep "✓ Latest version: $($releaseInfo.tag_name)"
        return $releaseInfo
    } catch {
        Write-Error "Failed to fetch release information from GitHub"
        Write-Error "Error: $($_.Exception.Message)"
        Write-Error "Please check your internet connection and try again"
        exit 1
    }
}

# Get download URL for Windows
function Get-DownloadUrl {
    param(
        [object]$ReleaseInfo,
        [string]$Architecture
    )
    
    $platform = "windows-$Architecture"
    $filename = "lil-rag-$platform.zip"
    
    Write-SubStep "Looking for release asset: $filename"
    
    $asset = $ReleaseInfo.assets | Where-Object { $_.name -eq $filename }
    
    if (-not $asset) {
        Write-Error "Could not find release asset for platform: $platform"
        Write-SubStep "Available assets:"
        foreach ($a in $ReleaseInfo.assets) {
            Write-SubStep "  $($a.name)"
        }
        exit 1
    }
    
    Write-SubStep "✓ Download URL: $($asset.browser_download_url)"
    return $asset.browser_download_url
}

# Download and install binaries
function Install-LilRag {
    param(
        [string]$DownloadUrl,
        [string]$Version
    )
    
    Write-Section "Downloading and Installing Lil-RAG $Version"
    
    $tempDir = Join-Path $env:TEMP "lil-rag-install-$(Get-Random)"
    $filename = Split-Path $DownloadUrl -Leaf
    $zipPath = Join-Path $tempDir $filename
    
    Write-SubStep "Creating temporary directory: $tempDir"
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
    
    Write-SubStep "Downloading $filename..."
    
    try {
        # Use WebClient for better progress reporting
        $webClient = New-Object System.Net.WebClient
        $webClient.DownloadFile($DownloadUrl, $zipPath)
        Write-SubStep "✓ Download completed"
    } catch {
        Write-Error "Failed to download $DownloadUrl"
        Write-Error "Error: $($_.Exception.Message)"
        Remove-Item $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        exit 1
    }
    
    # Verify download
    if (-not (Test-Path $zipPath) -or (Get-Item $zipPath).Length -eq 0) {
        Write-Error "Downloaded file is missing or empty"
        Remove-Item $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        exit 1
    }
    
    Write-SubStep "Extracting binaries..."
    
    try {
        # Extract ZIP file
        Add-Type -AssemblyName System.IO.Compression.FileSystem
        [System.IO.Compression.ZipFile]::ExtractToDirectory($zipPath, $tempDir)
    } catch {
        Write-Error "Failed to extract archive: $($_.Exception.Message)"
        Remove-Item $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        exit 1
    }
    
    # Find binaries
    $arch = Get-WindowsInfo
    $platform = "windows-$arch"
    $binaries = @()
    
    $binaryNames = @("lil-rag", "lil-rag-server", "lil-rag-mcp")
    foreach ($binary in $binaryNames) {
        $sourceName = "$binary-$platform.exe"
        $targetName = "$binary.exe"
        $sourcePath = Join-Path $tempDir $sourceName
        
        if (Test-Path $sourcePath) {
            $binaries += @{
                Source = $sourcePath
                Target = $targetName
            }
        }
    }
    
    if ($binaries.Count -eq 0) {
        Write-Error "No binaries found in the downloaded archive"
        Get-ChildItem $tempDir | ForEach-Object { Write-SubStep "  Found: $($_.Name)" }
        Remove-Item $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        exit 1
    }
    
    Write-SubStep "Found $($binaries.Count) binaries to install"
    
    # Create install directory
    if (-not (Test-Path $InstallDir)) {
        Write-SubStep "Creating install directory: $InstallDir"
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }
    
    # Install binaries
    foreach ($binary in $binaries) {
        $targetPath = Join-Path $InstallDir $binary.Target
        
        Write-SubStep "Installing $($binary.Target)..."
        
        try {
            Copy-Item $binary.Source $targetPath -Force
            Write-SubStep "✓ $($binary.Target) installed successfully"
        } catch {
            Write-Error "Failed to copy $($binary.Source) to $targetPath"
            Write-Error "Error: $($_.Exception.Message)"
            Remove-Item $tempDir -Recurse -Force -ErrorAction SilentlyContinue
            exit 1
        }
    }
    
    # Cleanup
    Remove-Item $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    Write-Success "Installation completed successfully"
}

# Verify installation
function Test-Installation {
    Write-Section "Verifying Installation"
    
    $binaries = @("lil-rag.exe", "lil-rag-server.exe", "lil-rag-mcp.exe")
    $allOk = $true
    
    foreach ($binary in $binaries) {
        $binaryPath = Join-Path $InstallDir $binary
        
        if (Test-Path $binaryPath) {
            # Test version command
            try {
                $versionOutput = & $binaryPath --version 2>$null
                if ($LASTEXITCODE -eq 0) {
                    Write-SubStep "✓ $binary`: $versionOutput"
                } else {
                    Write-SubStep "✓ $binary`: installed (version check unavailable)"
                }
            } catch {
                Write-SubStep "✓ $binary`: installed (version check failed)"
            }
        } else {
            Write-SubStep "✗ $binary`: not found"
            $allOk = $false
        }
    }
    
    if ($allOk) {
        Write-Success "All binaries installed and working correctly"
    } else {
        Write-Error "Some binaries failed verification"
        return $false
    }
    
    return $true
}

# Check PATH configuration
function Test-PathConfiguration {
    Write-Section "Checking PATH Configuration"
    
    $currentPath = $env:PATH
    if ($currentPath -like "*$InstallDir*") {
        Write-SubStep "✓ $InstallDir is already in PATH"
    } else {
        Write-Warning "$InstallDir is not in your PATH"
        Write-SubStep "To add it to your PATH:"
        Write-SubStep "  1. Open System Properties > Advanced > Environment Variables"
        Write-SubStep "  2. Edit PATH variable and add: $InstallDir"
        Write-SubStep "  3. Or run in PowerShell (as Administrator):"
        Write-SubStep "     [Environment]::SetEnvironmentVariable('PATH', `$env:PATH + ';$InstallDir', 'Machine')"
        Write-SubStep ""
        Write-SubStep "For current session only:"
        Write-SubStep "  `$env:PATH += ';$InstallDir'"
        Write-SubStep ""
        Write-SubStep "Or run commands with full path: $InstallDir\lil-rag.exe"
    }
}

# Post-installation setup
function Complete-Setup {
    Write-Section "Post-Installation Setup"
    
    Write-SubStep "Checking for Ollama..."
    $ollama = Get-Command ollama -ErrorAction SilentlyContinue
    if ($ollama) {
        Write-SubStep "✓ Ollama is installed"
        
        # Check if Ollama service is running
        $ollamaProcess = Get-Process ollama -ErrorAction SilentlyContinue
        if ($ollamaProcess) {
            Write-SubStep "✓ Ollama service is running"
        } else {
            Write-SubStep "⚠ Ollama service is not running"
            Write-SubStep "  Start it with: ollama serve"
        }
    } else {
        Write-Warning "Ollama is not installed"
        Write-SubStep "Install Ollama using one of these methods:"
        Write-SubStep "  1. Download from: https://ollama.ai"
        Write-SubStep "  2. Using Chocolatey: choco install ollama"
        Write-SubStep "  3. Using winget: winget install Ollama.Ollama"
    }
    
    # Windows-specific notes
    Write-SubStep "Windows Security Notes:"
    Write-SubStep "• If Windows Defender blocks execution, add $InstallDir to exclusions"
    Write-SubStep "• Some antivirus software may quarantine the binaries"
    Write-SubStep "• If you get 'execution policy' errors, run:"
    Write-SubStep "    Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser"
    
    Write-SubStep "Configuration and Data Locations:"
    Write-SubStep "• Configuration: %USERPROFILE%\.config\lil-rag\config.json"
    Write-SubStep "• Data storage: %USERPROFILE%\.local\share\lil-rag\"
    Write-SubStep "• Migration from %USERPROFILE%\.lilrag\ is automatic on first run"
    Write-SubStep ""
    Write-SubStep "Next steps:"
    Write-SubStep "1. Add $InstallDir to your PATH (see above)"
    Write-SubStep "2. Ensure Ollama is running: ollama serve"
    Write-SubStep "3. Pull an embedding model: ollama pull nomic-embed-text"
    Write-SubStep "4. Initialize lil-rag configuration: lil-rag config init"
    Write-SubStep "5. Index some content: lil-rag index doc1 'Your content here'"
    Write-SubStep "6. Search content: lil-rag search 'query'"
    Write-SubStep "7. Start web server: lil-rag-server"
}

# Main function
function Main {
    # Handle help and version flags
    if ($Help) {
        Show-Usage
        return
    }
    
    if ($Version) {
        Write-Host "Lil-RAG Windows Installation Script v2.0.0"
        return
    }
    
    Write-Host "🪟 Lil-RAG Windows Installation Script" -ForegroundColor $script:Colors.Purple
    Write-Host "=======================================" -ForegroundColor $script:Colors.Purple
    Write-Host ""
    
    # Detect system information
    $architecture = Get-WindowsInfo
    
    # Check if already installed
    if (-not $Force) {
        $existingInstall = Get-Command lil-rag -ErrorAction SilentlyContinue
        if ($existingInstall) {
            try {
                $installedVersion = & lil-rag --version 2>$null
                Write-Warning "lil-rag is already installed: $installedVersion"
                Write-Warning "Use -Force to reinstall"
                return
            } catch {
                # Version check failed, continue with installation
            }
        }
    }
    
    # System checks
    Test-SystemRequirements
    
    # Install dependencies
    if (-not $SkipDeps) {
        Install-Dependencies
    } else {
        Write-Log "Skipping dependency installation"
    }
    
    # Get release information
    $releaseInfo = Get-LatestRelease
    $version = $releaseInfo.tag_name
    
    # Get download URL
    $downloadUrl = Get-DownloadUrl -ReleaseInfo $releaseInfo -Architecture $architecture
    
    # Download and install
    Install-LilRag -DownloadUrl $downloadUrl -Version $version
    
    # Verify installation
    if (-not $SkipVerify) {
        $verifyResult = Test-Installation
        if (-not $verifyResult) {
            Write-Error "Installation verification failed"
            exit 1
        }
    } else {
        Write-Log "Skipping installation verification"
    }
    
    # Check PATH
    Test-PathConfiguration
    
    # Post-installation setup
    Complete-Setup
    
    Write-Host ""
    Write-Success "🎉 Lil-RAG installation completed successfully!"
    Write-Success "Version $version is now installed in $InstallDir"
    Write-Host ""
    
    # Final instructions
    Write-Section "Quick Start Guide"
    Write-SubStep "Visit the documentation: $script:GITHUB_REPO/$script:REPO#readme"
    Write-SubStep "Join the community for support and updates"
}

# Error handling
trap {
    Write-Error "An unexpected error occurred: $($_.Exception.Message)"
    Write-Error "Stack trace: $($_.ScriptStackTrace)"
    exit 1
}

# Run main function
Main