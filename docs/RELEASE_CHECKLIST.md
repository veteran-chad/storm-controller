# Release Checklist

This checklist should be followed when preparing a new release of the Storm Kubernetes Controller.

## Pre-Release Checklist

- [ ] All tests are passing on main branch
- [ ] No security vulnerabilities in latest scans
- [ ] Documentation is up to date
- [ ] CHANGELOG.md is updated with all changes
- [ ] Examples are tested and working

## Release Process

### 1. Prepare Release Branch

```bash
# Create release branch
git checkout -b release/v1.0.0

# Update version in documentation if needed
# Update CHANGELOG.md with release date
git commit -m "chore: prepare release v1.0.0"
```

### 2. Create Pull Request

- [ ] Create PR from release branch to main
- [ ] Ensure all CI checks pass
- [ ] Get approval from maintainer
- [ ] Merge PR

### 3. Tag Release

```bash
# Checkout main and pull latest
git checkout main
git pull origin main

# Create and push tag
git tag -a v1.0.0 -m "Release v1.0.0

- Feature: Add support for Storm 2.8.1
- Feature: Multi-Storm version support
- Fix: Improved topology deletion handling
- Docs: Comprehensive build and release documentation"

git push origin v1.0.0
```

### 4. Verify Automated Release

Monitor GitHub Actions for:
- [ ] Multi-Storm version images built successfully
- [ ] Images pushed to ghcr.io
- [ ] Helm chart packaged and pushed
- [ ] GitHub release created

### 5. Post-Release Verification

- [ ] Pull and test controller images:
  ```bash
  docker pull ghcr.io/veteran-chad/storm-controller:v1.0.0
  docker pull ghcr.io/veteran-chad/storm-controller:v1.0.0-storm2.6.4
  docker pull ghcr.io/veteran-chad/storm-controller:v1.0.0-storm2.8.1
  ```

- [ ] Install Helm chart:
  ```bash
  helm install test oci://ghcr.io/veteran-chad/charts/storm-kubernetes --version 1.0.0
  ```

- [ ] Verify GitHub release page has correct release notes

### 6. Announce Release

- [ ] Update project README with new version
- [ ] Create announcement for users
- [ ] Update any external documentation

## Hotfix Release Process

For urgent fixes:

1. Create hotfix branch from tag:
   ```bash
   git checkout -b hotfix/v1.0.1 v1.0.0
   ```

2. Apply fixes and commit

3. Tag and push:
   ```bash
   git tag -a v1.0.1 -m "Hotfix: <description>"
   git push origin v1.0.1
   ```

4. Cherry-pick fix to main:
   ```bash
   git checkout main
   git cherry-pick <commit-hash>
   git push origin main
   ```

## Release Notes Template

```markdown
## Storm Controller v1.0.0

### Highlights
- Brief summary of major changes

### Container Images
- `ghcr.io/veteran-chad/storm-controller:v1.0.0` (Storm 2.8.1)
- `ghcr.io/veteran-chad/storm-controller:v1.0.0-storm2.6.4`
- `ghcr.io/veteran-chad/storm-controller:v1.0.0-storm2.8.1`

### Installation

```bash
# Helm
helm install storm-kubernetes oci://ghcr.io/veteran-chad/charts/storm-kubernetes --version 1.0.0

# Or with specific Storm version
helm install storm-kubernetes oci://ghcr.io/veteran-chad/charts/storm-kubernetes \
  --version 1.0.0 \
  --set image.tag=v1.0.0-storm2.6.4
```

### Changes
- Feature: Description
- Fix: Description
- Docs: Description

### Contributors
- @username

### Full Changelog
https://github.com/veteran-chad/storm-controller/compare/v0.9.0...v1.0.0
```