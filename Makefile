# Makefile

# Define variables
APP_NAME := ftransfer
VERSION := 1.1.0
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILD_DIR := build

# Default target
all: build

# Ensure the build directory exists
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)


# Build the application
build: $(BUILD_DIR)
	go build -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)" -o $(BUILD_DIR)/$(APP_NAME)

# Clean the build
clean:
	rm -f $(APP_NAME)

# Run tests
test:
	go test -v ./...

# Run the application
run: build
	./$(APP_NAME)

# Print version information
version:
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"

.PHONY: all build clean run version