# Storm Controller Build and Release Plan

## Overview

This document outlines the comprehensive build, release, and versioning strategy for the Storm Kubernetes Controller project. The plan includes CI/CD pipelines, semantic versioning, multi-Storm version support, and security scanning.

## Versioning Strategy

### Semantic Versioning Format

```
v<MAJOR>.<MINOR>.<PATCH>[-<STORM_VERSION>][-<PRERELEASE>][+<BUILD>]
```

Examples:
- `v1.0.0` - Base controller version (defaults to latest supported Storm)
- `v1.0.0-storm2.6.4` - Controller version with specific Storm support
- `v1.0.0-storm2.8.1` - Same controller version supporting Storm 2.8.1
- `v1.0.0-rc.1` - Release candidate
- `v1.0.0-storm2.6.4-rc.1` - Release candidate for Storm 2.6.4

### Version Components

1. **Controller Version** (`MAJOR.MINOR.PATCH`)
   - MAJOR: Breaking changes to CRDs or controller behavior
   - MINOR: New features, backwards compatible
   - PATCH: Bug fixes, security updates

2. **Storm Version** (`storm<VERSION>`)
   - Indicates which Storm version the image is built with
   - Allows multiple Storm versions per controller release

3. **Pre-release** (`rc.X`, `beta.X`, `alpha.X`)
   - For testing releases before GA

## Tagging Strategy

### Git Tags
```bash
# Controller releases
v1.0.0                    # Main release tag
v1.0.0-storm2.6.4        # Storm-specific build tag
v1.0.0-storm2.8.1        # Another Storm version

# Pre-releases
v1.0.0-rc.1
v1.0.0-storm2.6.4-rc.1
```

### Container Image Tags

#### Storm Controller Image
```
ghcr.io/veteran-chad/storm-controller:<tag>
```

Tags:
- `latest` - Latest stable release with default Storm version
- `v1.0.0` - Specific release with default Storm version
- `v1.0.0-storm2.6.4` - Specific release with Storm 2.6.4
- `v1.0.0-storm2.8.1` - Specific release with Storm 2.8.1
- `main` - Latest main branch build (for development)
- `sha-<commit>` - Specific commit builds

#### Storm Controller JAR Image
```
ghcr.io/veteran-chad/storm-controller-jar:<tag>
```

Same tagging pattern as controller image.

#### Helm Chart
```
oci://ghcr.io/veteran-chad/charts/storm-kubernetes:<tag>
```

Tags:
- `1.0.0` - Chart version (may differ from controller version)
- `latest` - Latest stable chart

## Build Matrix

### Supported Storm Versions
```yaml
storm_versions:
  - "2.6.4"  # Current production version
  - "2.8.1"  # Latest stable version
  - "3.0.0"  # Future version (when released)
```

### Build Combinations
For each controller release, build:
1. One image per supported Storm version
2. One "default" image tagged without Storm version (uses latest stable)

## CI/CD Pipeline

### 1. Pull Request Checks

**File**: `.github/workflows/pr-checks.yml`

```yaml
name: PR Checks
on:
  pull_request:
    branches: [main]

jobs:
  lint:
    name: Lint Code
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v3
        with:
          working-directory: src
      
  test:
    name: Run Tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - name: Run tests
        working-directory: src
        run: |
          make test
          
  security-scan:
    name: Security Scanning
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      # Go vulnerability scanning
      - name: Run govulncheck
        uses: golang/govulncheck-action@v1
        with:
          working-directory: src
          
      # Container scanning with Trivy
      - name: Build test image
        run: |
          cd src
          docker build -t test-image:${{ github.sha }} .
          
      - name: Run Trivy vulnerability scanner
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: test-image:${{ github.sha }}
          format: 'sarif'
          output: 'trivy-results.sarif'
          
      # SAST with CodeQL
      - name: Initialize CodeQL
        uses: github/codeql-action/init@v2
        with:
          languages: go
          
      - name: Perform CodeQL Analysis
        uses: github/codeql-action/analyze@v2
        
  helm-lint:
    name: Helm Chart Linting
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Lint Helm charts
        run: |
          helm lint charts/storm-kubernetes
          
  validate-manifests:
    name: Validate Kubernetes Manifests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Validate CRDs and RBAC
        run: |
          cd src
          make manifests
          git diff --exit-code
```

### 2. Main Branch Build and Release

**File**: `.github/workflows/release.yml`

```yaml
name: Release
on:
  push:
    branches: [main]
    tags:
      - 'v*'

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  build-matrix:
    name: Build Multi-Storm Version Images
    runs-on: ubuntu-latest
    strategy:
      matrix:
        storm-version: ["2.6.4", "2.8.1"]
    permissions:
      contents: read
      packages: write
      
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0  # For git describe
          
      - name: Determine version
        id: version
        run: |
          if [[ $GITHUB_REF == refs/tags/v* ]]; then
            VERSION=${GITHUB_REF#refs/tags/}
          else
            VERSION=main-$(git rev-parse --short HEAD)
          fi
          echo "version=$VERSION" >> $GITHUB_OUTPUT
          echo "storm_tag=$VERSION-storm${{ matrix.storm-version }}" >> $GITHUB_OUTPUT
          
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3
        
      - name: Log in to Container Registry
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
          
      - name: Build and push controller image
        uses: docker/build-push-action@v5
        with:
          context: src
          push: true
          build-args: |
            STORM_VERSION=${{ matrix.storm-version }}
          tags: |
            ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ steps.version.outputs.storm_tag }}
            ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:storm${{ matrix.storm-version }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
          
      - name: Build and push JAR container image
        uses: docker/build-push-action@v5
        with:
          context: containers/storm-controller-topology-jar
          push: true
          build-args: |
            STORM_VERSION=${{ matrix.storm-version }}
          tags: |
            ${{ env.REGISTRY }}/${{ github.repository }}-jar:${{ steps.version.outputs.storm_tag }}
            ${{ env.REGISTRY }}/${{ github.repository }}-jar:storm${{ matrix.storm-version }}
            
  build-default:
    name: Build Default Storm Version
    runs-on: ubuntu-latest
    needs: build-matrix
    if: startsWith(github.ref, 'refs/tags/v')
    permissions:
      contents: read
      packages: write
      
    steps:
      - uses: actions/checkout@v4
      
      - name: Determine version
        id: version
        run: |
          VERSION=${GITHUB_REF#refs/tags/}
          echo "version=$VERSION" >> $GITHUB_OUTPUT
          
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3
        
      - name: Log in to Container Registry
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
          
      - name: Build and push controller image (default Storm)
        uses: docker/build-push-action@v5
        with:
          context: src
          push: true
          build-args: |
            STORM_VERSION=2.8.1
          tags: |
            ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ steps.version.outputs.version }}
            ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:latest
            
  release-helm-chart:
    name: Release Helm Chart
    runs-on: ubuntu-latest
    needs: [build-matrix, build-default]
    if: startsWith(github.ref, 'refs/tags/v')
    permissions:
      contents: read
      packages: write
      
    steps:
      - uses: actions/checkout@v4
      
      - name: Package Helm chart
        run: |
          VERSION=${GITHUB_REF#refs/tags/v}
          # Update Chart.yaml with version
          sed -i "s/version: .*/version: ${VERSION}/" charts/storm-kubernetes/Chart.yaml
          sed -i "s/appVersion: .*/appVersion: ${VERSION}/" charts/storm-kubernetes/Chart.yaml
          
          helm package charts/storm-kubernetes
          
      - name: Push to OCI registry
        run: |
          VERSION=${GITHUB_REF#refs/tags/v}
          helm push storm-kubernetes-${VERSION}.tgz oci://${{ env.REGISTRY }}/${{ github.repository_owner }}/charts
          
  create-release:
    name: Create GitHub Release
    runs-on: ubuntu-latest
    needs: [build-matrix, build-default, release-helm-chart]
    if: startsWith(github.ref, 'refs/tags/v')
    permissions:
      contents: write
      
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
          
      - name: Generate release notes
        id: notes
        run: |
          VERSION=${GITHUB_REF#refs/tags/}
          cat > release-notes.md << EOF
          ## Storm Controller ${VERSION}
          
          ### Container Images
          
          Controller images with Storm versions:
          - \`ghcr.io/veteran-chad/storm-controller:${VERSION}\` (Storm 2.8.1 - default)
          - \`ghcr.io/veteran-chad/storm-controller:${VERSION}-storm2.6.4\`
          - \`ghcr.io/veteran-chad/storm-controller:${VERSION}-storm2.8.1\`
          
          ### Helm Chart
          
          \`\`\`bash
          helm install storm-kubernetes oci://ghcr.io/veteran-chad/charts/storm-kubernetes --version ${VERSION#v}
          \`\`\`
          
          ### Changelog
          
          $(git log --pretty=format:"- %s" $(git describe --tags --abbrev=0 HEAD^)..HEAD)
          EOF
          
      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          body_path: release-notes.md
          draft: false
          prerelease: ${{ contains(github.ref, '-rc.') || contains(github.ref, '-beta.') }}
```

### 3. Scheduled Security Scanning

**File**: `.github/workflows/security-scan.yml`

```yaml
name: Scheduled Security Scan
on:
  schedule:
    - cron: '0 2 * * 1'  # Weekly on Monday at 2am UTC
  workflow_dispatch:

jobs:
  scan-images:
    name: Scan Published Images
    runs-on: ubuntu-latest
    strategy:
      matrix:
        image:
          - storm-controller
          - storm-controller-jar
        storm-version: ["2.6.4", "2.8.1"]
    steps:
      - name: Run Trivy scanner
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: ghcr.io/veteran-chad/${{ matrix.image }}:storm${{ matrix.storm-version }}
          format: 'sarif'
          output: 'trivy-${{ matrix.image }}-${{ matrix.storm-version }}.sarif'
          
      - name: Upload Trivy scan results
        uses: github/codeql-action/upload-sarif@v2
        with:
          sarif_file: 'trivy-${{ matrix.image }}-${{ matrix.storm-version }}.sarif'
```

## Release Process

### 1. Development Workflow
```bash
# Feature development
git checkout -b feature/new-feature
# Make changes
git commit -m "feat: add new feature"
git push origin feature/new-feature
# Create PR to main
```

### 2. Release Preparation
```bash
# Update version in relevant files
# Update CHANGELOG.md
git checkout -b release/v1.0.0
git commit -m "chore: prepare release v1.0.0"
git push origin release/v1.0.0
# Create PR to main
```

### 3. Creating a Release
```bash
# After PR is merged to main
git checkout main
git pull origin main
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

This triggers:
1. Multi-Storm version builds
2. Container image publishing
3. Helm chart publishing
4. GitHub release creation

### 4. Patch Releases
```bash
# For urgent fixes
git checkout -b hotfix/v1.0.1 main
# Make fixes
git commit -m "fix: critical bug fix"
git push origin hotfix/v1.0.1
# Create PR to main
# After merge, tag as v1.0.1
```

## Supporting Multiple Storm Versions

### 1. Dockerfile Multi-Stage Build
```dockerfile
# Build argument for Storm version
ARG STORM_VERSION=2.8.1

# Use specific Storm base image
FROM storm:${STORM_VERSION} as storm-base

# Controller build stage
FROM golang:1.21 as builder
# ... build controller ...

# Final stage
FROM storm-base
COPY --from=builder /workspace/manager /manager
# ... rest of Dockerfile ...
```

### 2. Helm Chart Values
```yaml
# values.yaml
image:
  repository: ghcr.io/veteran-chad/storm-controller
  tag: v1.0.0-storm2.8.1  # Default to specific Storm version
  pullPolicy: IfNotPresent

# Allow override for different Storm versions
storm:
  version: "2.8.1"
```

### 3. Compatibility Matrix

| Controller Version | Storm 2.6.x | Storm 2.8.x | Storm 3.0.x |
|-------------------|-------------|-------------|-------------|
| v1.0.x            | ✅          | ✅          | ❌          |
| v1.1.x            | ✅          | ✅          | ✅          |
| v2.0.x            | ❌          | ✅          | ✅          |

## Security Considerations

### 1. Image Scanning
- All images scanned with Trivy before publishing
- Weekly scans of published images
- Critical vulnerabilities block releases

### 2. Code Scanning
- CodeQL analysis on every PR
- Govulncheck for Go vulnerabilities
- Dependabot for dependency updates

### 3. Supply Chain Security
- SBOM generation for all images
- Image signing with cosign (future enhancement)
- Provenance attestation (future enhancement)

## Monitoring and Metrics

### Release Metrics to Track
1. Build success rate
2. Image pull counts
3. Security vulnerability counts
4. Time to build/release
5. Helm chart installation count

## Rollback Strategy

### Container Images
```bash
# Rollback to previous version
kubectl set image deployment/storm-controller \
  controller=ghcr.io/veteran-chad/storm-controller:v0.9.0-storm2.8.1
```

### Helm Chart
```bash
# Rollback Helm release
helm rollback storm-kubernetes <revision>
```

## Future Enhancements

1. **Multi-Architecture Builds**
   - Support for ARM64
   - Platform-specific optimizations

2. **Advanced Testing**
   - Integration tests with real Storm clusters
   - Chaos testing
   - Performance benchmarking

3. **Enhanced Security**
   - Image signing with Sigstore/Cosign
   - SLSA compliance
   - Runtime security policies

4. **Distribution**
   - Publish to additional registries (Docker Hub, Quay.io)
   - Helm chart repository hosting
   - Operator Hub listing

## Conclusion

This build and release plan provides:
- Clear versioning strategy supporting multiple Storm versions
- Automated CI/CD pipelines with security scanning
- Semantic versioning for predictable releases
- Comprehensive testing and validation
- Easy rollback capabilities

The plan ensures reliable, secure, and versioned releases of the Storm Kubernetes Controller across multiple Storm versions.