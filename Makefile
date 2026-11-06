BINARY  := navidrome-importer
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64

.PHONY: build test vet fmt clean dist

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w *.go

clean:
	rm -rf $(BINARY) $(BINARY).exe dist

# Build the binaries for every platform into dist/
dist:
	rm -rf dist
	mkdir -p dist
	@set -e; \
	for target in $(PLATFORMS); do \
		os=$${target%/*}; arch=$${target#*/}; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		out="dist/$(BINARY)-$$os-$$arch$$ext"; \
		echo "  build $$out"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
			go build -trimpath -ldflags "$(LDFLAGS)" -o "$$out" .; \
	done
	@cd dist && if command -v shasum >/dev/null 2>&1; then \
		shasum -a 256 * > SHA256SUMS; \
	else \
		sha256sum * > SHA256SUMS; \
	fi
	@echo "Done: binaries in dist/"
