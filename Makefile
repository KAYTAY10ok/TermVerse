BINARY  := termverse
TAG     := $(shell git describe --tags --exact-match 2>/dev/null | sed 's/^v//')
VERSION ?= $(if $(TAG),$(TAG),0.1.0)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
PKG     := github.com/KAYTAY10ok/TermVerse/internal/version
LDFLAGS := -s -w -X $(PKG).Version=$(VERSION) -X $(PKG).Commit=$(COMMIT) -X $(PKG).Date=$(DATE)
PREFIX  ?= $(HOME)/.local
GOARCH  ?= $(shell go env GOARCH)

.PHONY: build run test vet fmt lint install uninstall clean deb release-snapshot

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/termverse

run: build
	./$(BINARY)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

lint: vet
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:"; gofmt -l .; exit 1)

install: build
	install -d $(PREFIX)/bin
	install -m 0755 $(BINARY) $(PREFIX)/bin/$(BINARY)
	@echo "Installed to $(PREFIX)/bin/$(BINARY) (make sure it is in your PATH)"

uninstall:
	rm -f $(PREFIX)/bin/$(BINARY)

deb: build
	VERSION=$(VERSION) ARCH=$(GOARCH) ./packaging/deb/build-deb.sh ./$(BINARY)

release-snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -f $(BINARY) termverse_*.deb
	rm -rf dist
