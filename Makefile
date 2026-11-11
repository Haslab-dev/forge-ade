.PHONY: run build build-dev clean test vet package

BINARY := build/bin/forge-native
LDFLAGS_PROD := -s -w

all: build

run:
	go run ./cmd/forge-native

# Production release build (stripped of DWARF debug symbols and symbol tables)
build:
	@mkdir -p build/bin
	go build -ldflags="$(LDFLAGS_PROD)" -o $(BINARY) ./cmd/forge-native
	@ls -lh $(BINARY)

# Development build (with full DWARF symbols for debuggers and stacktraces)
build-dev:
	@mkdir -p build/bin
	go build -o $(BINARY) ./cmd/forge-native
	@ls -lh $(BINARY)

test:
	go test -v ./...

vet:
	go vet ./...

clean:
	rm -rf build/bin
