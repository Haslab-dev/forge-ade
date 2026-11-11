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
	@mkdir -p "$(APP)/Contents/MacOS"
	cp $(BINARY) "$(APP)/Contents/MacOS/forge-native"
	@printf '%s\n' \
		'<?xml version="1.0" encoding="UTF-8"?>' \
		'<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">' \
		'<plist version="1.0">' \
		'<dict>' \
		'	<key>CFBundleIdentifier</key><string>com.haslab.forge-native</string>' \
		'	<key>CFBundleName</key><string>forge-native</string>' \
		'	<key>CFBundleDisplayName</key><string>ForgeADE Native</string>' \
		'	<key>CFBundleExecutable</key><string>forge-native</string>' \
		'	<key>CFBundlePackageType</key><string>APPL</string>' \
		'	<key>CFBundleShortVersionString</key><string>0.1.0</string>' \
		'	<key>CFBundleVersion</key><string>1</string>' \
		'	<key>NSHighResolutionCapable</key><true/>' \
		'</dict>' > "$(APP)/Contents/Info.plist"
	@echo "Built $(APP)"

test:
	go test -v ./...

vet:
	go vet ./...

clean:
	rm -rf build/bin build/forge-native.app
