# Storm Controller Release Makefile

# Version information
VERSION ?= $(shell git describe --tags --always --dirty)
STORM_VERSIONS := 2.6.4 2.8.1
DEFAULT_STORM_VERSION := 2.8.1

# Registry and image names
REGISTRY ?= docker.io
ORG ?= veteranchad
CONTROLLER_IMAGE := $(REGISTRY)/$(ORG)/storm-controller
JAR_IMAGE := $(REGISTRY)/$(ORG)/storm-controller-jar
CHART_REPO := registry-1.docker.io/veteranchad

# Build variables
GOARCH ?= amd64
GOOS ?= linux

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Format Go code
	cd src && go fmt ./...

.PHONY: vet
vet: ## Run go vet
	cd src && go vet ./...

.PHONY: test
test: ## Run tests
	cd src && go test -v -race ./...

.PHONY: build
build: ## Build controller binary
	cd src && CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o bin/manager main.go

##@ Docker

.PHONY: docker-build
docker-build: ## Build controller image for all Storm versions
	@for storm_version in $(STORM_VERSIONS); do \
		echo "Building controller for Storm $$storm_version..."; \
		docker build -t $(CONTROLLER_IMAGE):$(VERSION)-storm$$storm_version \
			--build-arg STORM_VERSION=$$storm_version \
			src/; \
	done
	@echo "Building controller with default Storm version $(DEFAULT_STORM_VERSION)..."
	@docker build -t $(CONTROLLER_IMAGE):$(VERSION) \
		--build-arg STORM_VERSION=$(DEFAULT_STORM_VERSION) \
		src/

.PHONY: docker-push
docker-push: ## Push controller images
	@for storm_version in $(STORM_VERSIONS); do \
		echo "Pushing controller for Storm $$storm_version..."; \
		docker push $(CONTROLLER_IMAGE):$(VERSION)-storm$$storm_version; \
	done
	@docker push $(CONTROLLER_IMAGE):$(VERSION)
	@if [ "$(VERSION)" != *"-"* ]; then \
		docker tag $(CONTROLLER_IMAGE):$(VERSION) $(CONTROLLER_IMAGE):latest; \
		docker push $(CONTROLLER_IMAGE):latest; \
	fi

.PHONY: jar-image-build
jar-image-build: ## Build JAR container images
	@for storm_version in $(STORM_VERSIONS); do \
		echo "Building JAR container for Storm $$storm_version..."; \
		docker build -t $(JAR_IMAGE):$(VERSION)-storm$$storm_version \
			--build-arg STORM_VERSION=$$storm_version \
			containers/storm-controller-topology-jar/; \
	done

.PHONY: jar-image-push
jar-image-push: ## Push JAR container images
	@for storm_version in $(STORM_VERSIONS); do \
		echo "Pushing JAR container for Storm $$storm_version..."; \
		docker push $(JAR_IMAGE):$(VERSION)-storm$$storm_version; \
	done

##@ Helm

.PHONY: helm-deps
helm-deps: ## Build Helm chart dependencies
	helm dependency build charts/storm-kubernetes

.PHONY: helm-lint
helm-lint: helm-deps ## Lint Helm chart
	helm lint charts/storm-kubernetes

.PHONY: helm-package
helm-package: ## Package Helm chart
	@VERSION=$${VERSION#v}; \
	sed -i.bak "s/version: .*/version: $$VERSION/" charts/storm-kubernetes/Chart.yaml && \
	sed -i.bak "s/appVersion: .*/appVersion: \"$(VERSION)\"/" charts/storm-kubernetes/Chart.yaml && \
	helm package charts/storm-kubernetes && \
	mv charts/storm-kubernetes/Chart.yaml.bak charts/storm-kubernetes/Chart.yaml

.PHONY: helm-push
helm-push: helm-package ## Push Helm chart to OCI registry  
	@echo "Note: You must be logged in to Docker Hub with 'helm registry login registry-1.docker.io'"
	@VERSION=$${VERSION#v}; \
	helm push storm-kubernetes-$$VERSION.tgz oci://$(CHART_REPO)

##@ Release

.PHONY: release-images
release-images: docker-build docker-push jar-image-build jar-image-push ## Build and push all images

.PHONY: release
release: release-images helm-push ## Full release (images + helm chart)
	@echo "Release $(VERSION) completed!"
	@echo "Images pushed:"
	@for storm_version in $(STORM_VERSIONS); do \
		echo "  - $(CONTROLLER_IMAGE):$(VERSION)-storm$$storm_version"; \
		echo "  - $(JAR_IMAGE):$(VERSION)-storm$$storm_version"; \
	done
	@echo "  - $(CONTROLLER_IMAGE):$(VERSION) (default)"
	@echo "  - $(CONTROLLER_IMAGE):latest"
	@echo "Helm chart pushed:"
	@echo "  - oci://$(CHART_REPO)/storm-kubernetes:$${VERSION#v}"

.PHONY: tag
tag: ## Create a new version tag
	@if [ -z "$(VERSION)" ]; then \
		echo "VERSION is required. Usage: make tag VERSION=v1.0.0"; \
		exit 1; \
	fi
	@if git rev-parse $(VERSION) >/dev/null 2>&1; then \
		echo "Tag $(VERSION) already exists"; \
		exit 1; \
	fi
	@echo "Creating tag $(VERSION)..."
	@git tag -a $(VERSION) -m "Release $(VERSION)"
	@echo "Tag created. Push with: git push origin $(VERSION)"

##@ Cleanup

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf src/bin/
	rm -f storm-kubernetes-*.tgz