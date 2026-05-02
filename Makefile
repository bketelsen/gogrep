IMPORT_PATH  := github.com/bketelsen/gogrep
DOCKER_IMAGE := gogrep
BIN_DIR      := bin
BINARY       := gogrep

VERSION   := $(shell git describe --tags --always --dirty="-dev" 2>/dev/null || echo dev)
DATE      := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS   := -s -w -X main.Version=$(VERSION) -X main.BuildTime=$(DATE)

.PHONY: all build install test vet fmt cover clean docker release deps tidy

all: test build

build:
	@echo "Building $(BINARY) $(VERSION)..."
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags='$(LDFLAGS)' -o $(BIN_DIR)/$(BINARY) .

install:
	@echo "Installing..."
	go install -trimpath -ldflags='$(LDFLAGS)' .

test:
	@echo "Testing..."
	go test -race ./...

vet:
	@echo "Vetting..."
	go vet ./...

fmt:
	@echo "Formatting..."
	gofmt -w .

cover:
	@echo "Coverage..."
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

clean:
	@echo "Cleaning..."
	rm -rf $(BIN_DIR) coverage.out

docker:
	@echo "Docker build..."
	docker build \
	    --build-arg VERSION=$(VERSION) \
	    --build-arg BUILD_TIME=$(DATE) \
	    -t $(DOCKER_IMAGE):$(VERSION) \
	    -t $(DOCKER_IMAGE):latest .

release:
	@echo "Releasing..."
	goreleaser release --clean

deps tidy:
	@echo "Tidying modules..."
	go mod tidy
