.PHONY: build run test clean docker-build docker-run migrate-up migrate-down lint

# Go related variables
BINARY_NAME=rpulse
MAIN_PACKAGE=./cmd/server

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOLINT=golangci-lint

# Build the application
build:
	$(GOBUILD) -o $(BINARY_NAME) $(MAIN_PACKAGE)

# Run the application
run:
	$(GORUN) $(MAIN_PACKAGE)

# Run tests
test:
	$(GOTEST) ./...

# Clean build files
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

# Build docker image
docker-build:
	docker build -t $(BINARY_NAME) .

# Run docker container
docker-run:
	docker compose --profile local up

docker-run-remote:
	docker compose --profile remote up

# Install dependencies
deps:
	$(GOGET) -v ./...

# Run linter
lint:
	$(GOLINT) run

# Default target
all: clean build