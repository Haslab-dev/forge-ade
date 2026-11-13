.PHONY: run build build-dev app clean test vet package

BINARY := build/bin/forge-native
APP := build/forge-native.app
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

# macOS .app bundle (stable bundle identity for the dock, dialogs and automation)
app: build
	@mkdir -p "$(APP)/Contents/MacOS" "$(APP)/Contents/Resources"
	cp $(BINARY) "$(APP)/Contents/MacOS/forge-native"
	cp packaging/Info.plist "$(APP)/Contents/Info.plist"
	@codesign --force -s - "$(APP)" >/dev/null 2>&1 || true
	@echo "Built $(APP)"

test:
	go test -v ./...

vet:
	go vet ./...

clean:
	rm -rf build/bin build/forge-native.app
