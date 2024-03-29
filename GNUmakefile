GO ?= go
GOROOT := $(shell $(GO) env GOROOT)
export PATH := $(GOROOT)/bin:$(PATH)
export CGO_ENABLED=0
GOFLAGS ?= -v -trimpath -ldflags="$(VERSION_FLAGS)"
SOURCES := $(wildcard *.go)
VERSION := $(shell git describe --always --dirty)
GIT_BRANCH := $(shell git branch --show-current)
GIT_COMMIT := $(shell git log -1 | base64 -w 0)
BUILD_STAMP := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
VERSION_FLAGS := -X 'main.buildstamp=$(BUILD_STAMP)' \
	-X 'main.version=$(VERSION)' \
	-X 'main.gitCommit=$(GIT_COMMIT)'

all: postgang test lint

postgang: $(SOURCES)
	$(GO) build $(GOFLAGS)
	strip $@

.PHONY: test
test:
	$(GO) test ./...

.PHONY: lint
lint:
	$(GO) vet ./...
	set -e;\
	output=$$(git ls-files -z '*.go' | xargs -0 gofmt -d);\
	if [ -n "$$output" ]; then \
	  printf '%s\n' "$$output";\
	  exit 1;\
	fi


.PHNONY: clean
clean:
	$(GO) clean

.PHONY:format
format:
	git ls-files -z '*.go' | xargs -0 gofmt -w -s
