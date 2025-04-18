BINARY_NAME=gitsnip
BINARY_PATH=bin/$(BINARY_NAME)
CMD_PATH=./cmd/gitsnip

GOFLAGS ?=
TEST_FLAGS ?= -v

.PHONY: all build clean run run-build 

all: build

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	go build $(GOFLAGS) -o $(BINARY_PATH) $(CMD_PATH)

run:
	@echo "Running $(CMD_PATH) $(filter-out $@,$(MAKECMDGOALS))..."
	go run $(CMD_PATH) $(filter-out $@,$(MAKECMDGOALS))

run-build: build
	@echo "Running $(BINARY_PATH) $(filter-out $@,$(MAKECMDGOALS))..."
	./$(BINARY_PATH) $(filter-out $@,$(MAKECMDGOALS))

run-binary:
	@echo "Running $(BINARY_PATH) $(filter-out $@,$(MAKECMDGOALS))..."
	./$(BINARY_PATH) $(filter-out $@,$(MAKECMDGOALS))

clean:
	@echo "Cleaning..."
	rm -rf $(BINARY_PATH)

lint:
	@echo "Linting..."
	go fmt ./...