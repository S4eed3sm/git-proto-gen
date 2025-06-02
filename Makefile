VERSION=0.0.2
BINARY=git-proto-gen
BUILD_DIR=releases

# Build flags
LDFLAGS=-ldflags="-s -w"
GOFLAGS=CGO_ENABLED=0

# Platforms and architectures
PLATFORMS=linux darwin windows
ARCHS=amd64 arm64
NAMES=x86_64 arm64

# Create build directory
$(shell mkdir -p $(BUILD_DIR))

# Build for all platforms
.PHONY: all
all: clean $(foreach platform,$(PLATFORMS),$(foreach arch,$(ARCHS),build-$(platform)-$(arch)))

# Clean build directory
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)/*

# Build for specific platform and architecture
.PHONY: build-%
build-%:
	$(eval platform=$(word 1,$(subst -, ,$*)))
	$(eval arch=$(word 2,$(subst -, ,$*)))
	$(eval name=$(if $(filter amd64,$(arch)),x86_64,arm64))
	$(eval output=$(BUILD_DIR)/$(BINARY)-$(VERSION)-$(platform)-$(name))
	$(eval output=$(if $(filter windows,$(platform)),$(output).exe,$(output)))
	@echo "Building for $(platform)/$(arch)..."
	$(GOFLAGS) GOOS=$(platform) GOARCH=$(arch) go build $(LDFLAGS) -o $(output) .

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all        - Build for all platforms and architectures"
	@echo "  clean      - Remove all build artifacts"
	@echo "  build-*    - Build for specific platform and architecture (e.g., build-linux-amd64)"
	@echo ""
	@echo "Example usage:"
	@echo "  make all                    # Build for all platforms"
	@echo "  make build-linux-amd64      # Build for Linux x86_64"
	@echo "  make build-darwin-arm64     # Build for macOS ARM64"
	@echo "  make build-windows-amd64    # Build for Windows x86_64" 