GO ?= go
VERSION ?= dev
DIST_DIR ?= dist

.PHONY: all fmt fmt-check vet test build release-check clean

all: release-check

fmt:
	gofmt -w cmd internal

fmt-check:
	@test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal && exit 1)

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

build:
	$(GO) build -trimpath -ldflags="-X github.com/rioriost/Xanship/internal/xanship.version=$(VERSION)" ./cmd/xanship

release-check:
	./scripts/release-check.sh

clean:
	rm -rf $(DIST_DIR) ./xanship
