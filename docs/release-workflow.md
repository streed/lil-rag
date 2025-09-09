# Release Workflow Testing Guide

## Overview
The new tag-based release workflow ensures that releases use the exact version from the VERSION file.

## How It Works

### 1. Automatic Builds (Main Branch)
- Triggered on pushes to `main` branch
- Auto-increments patch version in VERSION file  
- Creates development releases with new version

### 2. Official Releases (Tags)
- Triggered on tag creation (v*.*.* pattern)
- Uses exact version from VERSION file
- Validates tag matches VERSION file content
- Creates official GitHub releases

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

- **Single Source of Truth**: VERSION file controls all release versions
- **Prevents Version Drift**: Tag must match VERSION file or build fails
- **Automated Quality**: Comprehensive build and validation pipeline
- **Documentation Integration**: Can read release notes from CHANGELOG.md