# Release Workflow Testing Guide

## Overview
The unified build and release workflow handles both development builds and official releases from a single GitHub Actions workflow file.

## How It Works

The `build-and-release.yml` workflow automatically detects the build type and adjusts its behavior accordingly:

### 1. Automatic Builds (Main Branch)
- **Trigger**: Pushes to `main` branch
- **Version Management**: Auto-increments patch version in VERSION file  
- **Release Type**: Creates prerelease with auto-generated version
- **Artifact Retention**: 30 days

### 2. Official Releases (Tags)
- **Trigger**: Tag creation (v*.*.* pattern)
- **Version Management**: Uses exact version from VERSION file
- **Validation**: Ensures tag matches VERSION file content
- **Release Type**: Creates official GitHub releases
- **Artifact Retention**: 90 days

### 3. Unified Build Process
Both build types use the same:
- Cross-platform build matrix (Linux, macOS, Windows)
- Native platform compilation with CGO support
- Binary versioning and archiving
- Checksum generation

## Testing the Release Workflow

### Step 1: Check Current Version
```bash
cat VERSION
# Should show: 1.0.22
```

### Step 2: Create a Test Tag (Matching VERSION)
```bash
# This will work - tag matches VERSION file
git tag v1.0.22
git push origin v1.0.22
```

### Step 3: Create a Test Tag (Mismatched Version)
```bash
# This will fail validation - tag doesn't match VERSION file
git tag v1.0.23
git push origin v1.0.23
# Workflow will fail with validation error
```

### Step 4: Proper Release Process
```bash
# 1. Update VERSION file for new release
echo "1.1.0" > VERSION

# 2. Update CHANGELOG.md with release notes

# 3. Commit the version change
git add VERSION CHANGELOG.md
git commit -m "Prepare release v1.1.0"

# 4. Create matching tag
git tag v1.1.0
git push origin main v1.1.0
```

## Validation Features

The workflow includes several validation checks:

1. **Version Format**: Tag must follow v*.*.* pattern
2. **Version Matching**: Tag version must exactly match VERSION file content
3. **Build Verification**: All binaries include correct version information
4. **Cross-Platform**: Builds work on all supported platforms

## Benefits

- **Unified Workflow**: Single GitHub Actions workflow handles both dev and release builds
- **Single Source of Truth**: VERSION file controls all release versions
- **Consistent Builds**: Same build matrix and process for all platforms
- **Prevents Version Drift**: Tag must match VERSION file or build fails
- **Automated Quality**: Comprehensive build and validation pipeline
- **Efficient Resource Usage**: No duplication of build logic
- **Documentation Integration**: Can read release notes from CHANGELOG.md

## Workflow Features

### Conditional Logic
- **Main Branch**: Auto-increments version, creates prerelease
- **Tag**: Validates version match, creates official release

### Build Matrix
- **Linux**: AMD64, ARM64 (built on Ubuntu runners)
- **macOS**: AMD64 (Intel), ARM64 (Apple Silicon) (built on macOS runners)  
- **Windows**: AMD64 (built on Windows runners with MinGW)

### Quality Assurance
- **Tests**: Run on Linux AMD64 builds
- **Version Verification**: All binaries include embedded version information
- **Cross-compilation**: Native builds avoid CGO issues
- **Checksums**: SHA256 checksums for all release archives