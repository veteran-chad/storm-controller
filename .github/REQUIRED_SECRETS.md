# Required GitHub Actions Secrets

This document lists all the secrets that need to be configured in your GitHub repository for the CI/CD workflows to function properly.

## Docker Hub Secrets

The following secrets are required for pushing Helm charts to Docker Hub OCI registry:

- **`DOCKERHUB_USERNAME`**: Your Docker Hub username
- **`DOCKERHUB_TOKEN`**: Docker Hub access token (not password)
  - Create at: https://hub.docker.com/settings/security
  - Required permissions: Read, Write

## Optional Secrets

These secrets are optional but may enhance functionality:

- **`CODECOV_TOKEN`**: Token for uploading code coverage reports to Codecov
  - Only required if you want code coverage reporting
  - Get from: https://app.codecov.io/github/YOUR_REPO/settings

## Setting up Secrets

1. Go to your repository on GitHub
2. Navigate to Settings → Secrets and variables → Actions
3. Click "New repository secret"
4. Add each secret with the appropriate name and value

## Verifying Secrets

You can verify your secrets are properly configured by:

1. Triggering a workflow run on a branch
2. Checking the workflow logs for successful authentication steps
3. Ensuring Helm charts are pushed to Docker Hub after merging to main