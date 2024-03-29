GOCMD=go
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
BINARY_NAME=slowql-digest
BUILD_ROOT=./bin/

GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
WHITE  := $(shell tput -Txterm setaf 7)
RESET  := $(shell tput -Txterm sgr0)

.PHONY: all digest replayer

all: help

## Build:
digest: ## Build slowql-digest
	$(GOCMD) mod tidy
	@GO111MODULE=on $(GOCMD) build -o $(BUILD_ROOT)digest ./cmd/slowql-digest/
	@echo "${GREEN}[*]${RESET} digest successfully built in ${YELLOW}${BUILD_ROOT}digest${RESET}"

replayer: ## Build slowql-replayer
	$(GOCMD) mod tidy
	@GO111MODULE=on $(GOCMD) build -o $(BUILD_ROOT)replayer ./cmd/slowql-replayer/
	@echo "${GREEN}[*]${RESET} replayer successfully built in ${YELLOW}${BUILD_ROOT}replayer${RESET}"

clean: ## Clean all the files and binaries generated by the Makefile
	$(GOCMD) mod tidy
	rm -rf $(BUILD_ROOT)
	rm -f ./profile.cov

## Test:
test: ## Run the tests of the project
	$(GOTEST) -v -race ./...

coverage: ## Run the tests of the project and export the coverage
	$(GOTEST) -cover -covermode=count -coverprofile=profile.cov ./...
	$(GOCMD) tool cover -func profile.cov

## Help:
help: ## Show this help
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} { \
		if (/^[a-zA-Z_-]+:.*?##.*$$/) {printf "    ${YELLOW}%-20s${GREEN}%s${RESET}\n", $$1, $$2} \
		else if (/^## .*$$/) {printf "  ${CYAN}%s${RESET}\n", substr($$1,4)} \
		}' $(MAKEFILE_LIST)

