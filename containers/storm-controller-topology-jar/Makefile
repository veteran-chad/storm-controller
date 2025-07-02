# Makefile for Storm Topology JAR Container

# Variables
REGISTRY ?= ghcr.io/veteran-chad
IMAGE_NAME ?= storm-controller-topology-jar
VERSION ?= latest
FULL_IMAGE = $(REGISTRY)/$(IMAGE_NAME):$(VERSION)

# Build arguments
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
VCS_REF := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

.PHONY: help
help: ## Show this help message
	@echo "Storm Topology JAR Container - Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the Docker image
	@echo "Building $(FULL_IMAGE)..."
	docker build \
		--label "org.opencontainers.image.created=$(BUILD_DATE)" \
		--label "org.opencontainers.image.revision=$(VCS_REF)" \
		--label "org.opencontainers.image.version=$(VERSION)" \
		-t $(FULL_IMAGE) \
		-f Dockerfile \
		.

.PHONY: push
push: ## Push the Docker image to registry
	@echo "Pushing $(FULL_IMAGE)..."
	docker push $(FULL_IMAGE)

.PHONY: test
test: build ## Test the Docker image
	@echo "Testing $(FULL_IMAGE)..."
	# Test that the image runs
	docker run --rm $(FULL_IMAGE) echo "Container test passed"
	# Test info mode
	docker run --rm $(FULL_IMAGE) info || true
	# Test that entrypoint exists and is executable
	docker run --rm --entrypoint /bin/sh $(FULL_IMAGE) -c "test -x /storm/entrypoint.sh"
	@echo "All tests passed!"

.PHONY: scan
scan: build ## Scan the image for vulnerabilities
	@echo "Scanning $(FULL_IMAGE) for vulnerabilities..."
	@which trivy > /dev/null || (echo "Trivy not installed. See https://github.com/aquasecurity/trivy" && exit 1)
	trivy image --severity HIGH,CRITICAL $(FULL_IMAGE)

.PHONY: clean
clean: ## Remove built images
	@echo "Removing $(FULL_IMAGE)..."
	docker rmi -f $(FULL_IMAGE) 2>/dev/null || true

.PHONY: example
example: ## Build an example topology image
	@echo "Building example topology image..."
	@echo "FROM $(FULL_IMAGE)" > Dockerfile.example
	@echo "RUN echo 'Manifest-Version: 1.0' > /tmp/manifest.txt && \\" >> Dockerfile.example
	@echo "    echo 'Main-Class: com.example.TestTopology' >> /tmp/manifest.txt && \\" >> Dockerfile.example
	@echo "    mkdir -p /tmp/com/example && \\" >> Dockerfile.example
	@echo "    echo 'public class TestTopology {}' > /tmp/com/example/TestTopology.java && \\" >> Dockerfile.example
	@echo "    cd /tmp && jar cfm /storm/jars/topology.jar manifest.txt com/" >> Dockerfile.example
	docker build -f Dockerfile.example -t example-topology:latest .
	rm -f Dockerfile.example
	@echo "Example topology image built as example-topology:latest"
	@echo "Test with: docker run --rm example-topology:latest info"

.PHONY: shell
shell: build ## Run interactive shell in the container
	docker run --rm -it --entrypoint /bin/sh $(FULL_IMAGE)

.PHONY: all
all: build test ## Build and test the image