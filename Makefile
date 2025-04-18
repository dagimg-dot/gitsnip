BINARY_NAME=gitsnip
BINARY_PATH=bin/$(BINARY_NAME)
CMD_PATH=./cmd/gitsnip

GOFLAGS ?=
TEST_FLAGS ?= -v

VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
BUILD_BY ?= $(shell whoami)

LDFLAGS := -s -w \
  -X github.com/dagimg-dot/gitsnip/internal/cli.version=$(VERSION) \
  -X github.com/dagimg-dot/gitsnip/internal/cli.commit=$(COMMIT) \
  -X github.com/dagimg-dot/gitsnip/internal/cli.buildDate=$(BUILD_DATE) \
  -X github.com/dagimg-dot/gitsnip/internal/cli.builtBy=$(BUILD_BY)

.PHONY: all build clean run run-build lint lint-fix setup-hooks release

all: build

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	go build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BINARY_PATH) $(CMD_PATH)

run:
	@echo "Running $(CMD_PATH) $(filter-out $@,$(MAKECMDGOALS))..."
	go run $(GOFLAGS) $(CMD_PATH) $(filter-out $@,$(MAKECMDGOALS))

run-build: build
	@echo "Running $(BINARY_PATH) $(filter-out $@,$(MAKECMDGOALS))..."
	./$(BINARY_PATH) $(filter-out $@,$(MAKECMDGOALS))

run-binary:
	@echo "Running $(BINARY_PATH) $(filter-out $@,$(MAKECMDGOALS))..."
	./$(BINARY_PATH) $(filter-out $@,$(MAKECMDGOALS))

clean:
	@echo "Cleaning..."
	rm -rf $(BINARY_PATH)
	rm -rf dist/*

lint:
	@echo "Linting..."
	go fmt ./...

release:
	@echo "Bumping version..."
	git tag v$(VERSION)
	git push origin v$(VERSION)

local-release: 
	@echo "Creating release for $(VERSION)..."
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o dist/$(BINARY_NAME)_linux_$(VERSION)_$(shell go env GOARCH) $(CMD_PATH)