.PHONY: all build server client clean install test deps

# Binary names
SERVER_BIN = vpn-server
CLIENT_BIN = vpn-client

# Build directory
BUILD_DIR = bin

# Go parameters
GOCMD = go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOGET = $(GOCMD) get
GOMOD = $(GOCMD) mod

# Build flags
LDFLAGS = -w -s

all: deps build

deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

build: server client

server:
	@echo "Building server..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(SERVER_BIN) ./cmd/server

client:
	@echo "Building client..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(CLIENT_BIN) ./cmd/client

# Build for multiple platforms
build-all: build-linux build-darwin build-windows

build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)/linux
	GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/linux/$(SERVER_BIN) ./cmd/server
	GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/linux/$(CLIENT_BIN) ./cmd/client

build-darwin:
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)/darwin
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/darwin/$(SERVER_BIN) ./cmd/server
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/darwin/$(CLIENT_BIN) ./cmd/client

build-windows:
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)/windows
	GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/windows/$(SERVER_BIN).exe ./cmd/server
	GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/windows/$(CLIENT_BIN).exe ./cmd/client

test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

install: build
	@echo "Installing binaries..."
	sudo install -m 0755 $(BUILD_DIR)/$(SERVER_BIN) /usr/local/bin/
	sudo install -m 0755 $(BUILD_DIR)/$(CLIENT_BIN) /usr/local/bin/

uninstall:
	@echo "Uninstalling binaries..."
	sudo rm -f /usr/local/bin/$(SERVER_BIN)
	sudo rm -f /usr/local/bin/$(CLIENT_BIN)

run-server:
	@echo "Running server..."
	$(BUILD_DIR)/$(SERVER_BIN)

run-client:
	@echo "Running client..."
	sudo $(BUILD_DIR)/$(CLIENT_BIN)
