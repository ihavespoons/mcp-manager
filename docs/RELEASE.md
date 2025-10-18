# Release Process

This document describes the release process for mcp-manager, including GitHub Actions workflows and Homebrew tap setup.

## Overview

The release process is fully automated using GitHub Actions:

1. **Push a version tag** (e.g., `v0.1.0`) to trigger the release
2. **GitHub Actions builds** binaries for all platforms
3. **GitHub Release created** with all artifacts and checksums
4. **Homebrew tap updated** automatically with the new version

## Prerequisites

### 1. Homebrew Tap Repository

Create a separate GitHub repository for the Homebrew tap:

```bash
# Repository naming convention: homebrew-{package-name}
# Example: ihavespoons/homebrew-mcp-manager
```

The tap repository will contain:
```
homebrew-mcp-manager/
├── README.md
└── Formula/
    └── mcp-manager.rb
```

### 2. GitHub Secrets

Configure the following secret in the main repository (ihavespoons/mcp-manager):

- `HOMEBREW_TAP_TOKEN`: GitHub Personal Access Token with `repo` scope for updating the tap repository

To create the token:
1. Go to GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Generate new token with `repo` scope
3. Add as repository secret: Settings → Secrets and variables → Actions → New repository secret

## Release Workflows

### CI Workflow (`.github/workflows/ci.yml`)

Runs on every push to main and on pull requests:

- **Test**: Runs `go vet` and `go test` with race detection and coverage
- **Build**: Builds for all platforms (linux, darwin, windows) × (amd64, arm64)
- **Format**: Checks code formatting with `gofmt`

### Release Workflow (`.github/workflows/release.yml`)

Triggers on version tags (e.g., `v*`):

#### Job 1: Build
- Builds binaries for 5 platforms:
  - linux-amd64
  - linux-arm64
  - darwin-amd64 (Intel Mac)
  - darwin-arm64 (Apple Silicon)
  - windows-amd64
- Creates tar.gz archives (Unix) or zip archives (Windows)
- Generates SHA256 checksums for each archive
- Uploads artifacts for the release job

#### Job 2: Release
- Downloads all build artifacts
- Extracts checksums for Homebrew formula
- Creates GitHub Release with:
  - All binary archives
  - All checksum files
  - Auto-generated release notes
- Outputs version and checksums for Homebrew job

#### Job 3: Update Homebrew
- Clones the Homebrew tap repository
- Generates `Formula/mcp-manager.rb` with:
  - Version number
  - Download URLs for all platforms
  - SHA256 checksums
  - Installation and test instructions
- Commits and pushes to tap repository

## Creating a Release

### Step 1: Prepare the release

Ensure the code is ready:
```bash
# Run tests
make test

# Check formatting
make fmt

# Build locally
make build
```

### Step 2: Create and push a tag

```bash
# Create an annotated tag
git tag -a v0.1.0 -m "Release v0.1.0"

# Push the tag
git push origin v0.1.0
```

### Step 3: Monitor the workflow

1. Go to GitHub Actions tab
2. Watch the "Release" workflow run
3. Verify all jobs complete successfully:
   - ✅ Build (5 matrix jobs)
   - ✅ Release
   - ✅ Update Homebrew Tap

### Step 4: Verify the release

1. Check GitHub Releases page for the new release
2. Verify all artifacts are present:
   - 5 `.tar.gz` or `.zip` files
   - 5 `.sha256` checksum files
3. Check Homebrew tap repository:
   - New commit updating `Formula/mcp-manager.rb`
   - Formula has correct version and checksums

### Step 5: Test installation

```bash
# Install from Homebrew tap
brew tap ihavespoons/mcp-manager
brew install mcp-manager

# Verify version
mcp-manager --version

# Test functionality
mcp-manager validate --config mcp-config.yaml
```

## Homebrew Tap Setup

### Initial Setup

1. **Create the tap repository**:
   ```bash
   mkdir homebrew-mcp-manager
   cd homebrew-mcp-manager
   git init
   mkdir Formula
   ```

2. **Create README.md**:
   ```markdown
   # Homebrew Tap for mcp-manager

   ## Installation

   \`\`\`bash
   brew tap ihavespoons/mcp-manager
   brew install mcp-manager
   \`\`\`

   ## Updating

   \`\`\`bash
   brew upgrade mcp-manager
   \`\`\`
   ```

3. **Push to GitHub**:
   ```bash
   git add .
   git commit -m "Initial commit"
   git remote add origin https://github.com/ihavespoons/homebrew-mcp-manager.git
   git push -u origin main
   ```

4. **Configure the token**:
   - Add `HOMEBREW_TAP_TOKEN` secret to mcp-manager repository

### Formula Structure

The GitHub Actions workflow automatically generates the formula with this structure:

```ruby
class McpManager < Formula
  desc "MCP (Model Context Protocol) server manager with Docker support"
  homepage "https://github.com/ihavespoons/mcp-manager"
  version "0.1.0"
  license "FSL-1.1-MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/ihavespoons/mcp-manager/releases/download/v0.1.0/mcp-manager-v0.1.0-darwin-arm64.tar.gz"
      sha256 "abc123..."
    else
      url "https://github.com/ihavespoons/mcp-manager/releases/download/v0.1.0/mcp-manager-v0.1.0-darwin-amd64.tar.gz"
      sha256 "def456..."
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/ihavespoons/mcp-manager/releases/download/v0.1.0/mcp-manager-v0.1.0-linux-arm64.tar.gz"
      sha256 "ghi789..."
    else
      url "https://github.com/ihavespoons/mcp-manager/releases/download/v0.1.0/mcp-manager-v0.1.0-linux-amd64.tar.gz"
      sha256 "jkl012..."
    end
  end

  def install
    bin.install "mcp-manager"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/mcp-manager --version")
  end
end
```

## Version Numbering

Follow semantic versioning (semver):

- **Major version** (v1.0.0): Breaking changes
- **Minor version** (v0.1.0): New features, backwards compatible
- **Patch version** (v0.0.1): Bug fixes, backwards compatible

Examples:
- `v0.1.0` - First release
- `v0.1.1` - Bug fix
- `v0.2.0` - New feature
- `v1.0.0` - Stable release

## Troubleshooting

### Release workflow fails

**Problem**: Build job fails
- Check Go version compatibility
- Verify cross-compilation works locally
- Check build logs for errors

**Problem**: Release creation fails
- Ensure `GITHUB_TOKEN` has correct permissions
- Verify artifacts were uploaded correctly

**Problem**: Homebrew update fails
- Check `HOMEBREW_TAP_TOKEN` is configured and valid
- Verify tap repository exists and is accessible
- Check tap repository branch is `main`

### Formula doesn't work

**Problem**: Wrong checksums
- Delete the release and tag
- Fix the issue
- Create a new release with a new version

**Problem**: Binary doesn't run
- Check the binary was built for correct platform
- Verify the archive extraction works
- Test the binary locally before releasing

## Future Enhancements

### SLSA3 Provenance

To add SLSA3 compliance (supply-chain security):

1. Add provenance generation job:
   ```yaml
   provenance:
     needs: [build]
     permissions:
       actions: read
       id-token: write
       contents: write
     uses: slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml@v2.0.0
   ```

2. Update release job to include provenance attestation

3. Document verification process for users

Resources:
- [SLSA Framework](https://slsa.dev/)
- [GitHub SLSA Generator](https://github.com/slsa-framework/slsa-github-generator)
- [GitHub Blog: SLSA3 with Actions](https://github.blog/security/supply-chain-security/slsa-3-compliance-with-github-actions/)

### Additional Improvements

- [ ] Add code signing for macOS binaries
- [ ] Notarize macOS binaries for Gatekeeper
- [ ] Add Windows code signing
- [ ] Publish to additional package managers (apt, rpm, chocolatey)
- [ ] Add Docker images to GitHub Container Registry
- [ ] Automate changelog generation
