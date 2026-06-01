GO ?= go
GOLANGCI_LINT ?= golangci-lint

GO_FILES := $(shell find . -name '*.go' -not -path './.git/*')

.PHONY: help fmt fmt-check tidy tidy-check vet lint test race ci

help:
	@printf '%s\n' \
		'Targets:' \
		'  fmt         Format Go sources with gofmt' \
		'  fmt-check   Fail when Go sources are not gofmt-formatted' \
		'  tidy        Run go mod tidy' \
		'  tidy-check  Run go mod tidy and fail on go.mod/go.sum drift' \
		'  vet         Run go vet ./...' \
		'  lint        Run golangci-lint' \
		'  test        Run uncached go test ./... including Testcontainers tests' \
		'  race        Run uncached go test -race ./... including Testcontainers tests' \
		'  ci          Run the local CI gate'

fmt:
	@gofmt -w $(GO_FILES)

fmt-check:
	@test -z "$$(gofmt -l $(GO_FILES))"

tidy:
	@$(GO) mod tidy

tidy-check:
	@$(GO) mod tidy
	@git diff --exit-code -- go.mod go.sum

vet:
	@$(GO) vet ./...

lint:
	@$(GOLANGCI_LINT) run ./...

test:
	@$(GO) test -count=1 ./...

race:
	@$(GO) test -race -count=1 ./...

ci: tidy-check fmt-check vet lint test race

