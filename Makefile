.PHONY: run build clean test vet

BINARY := build/bin/forge-native

all: build

run:
	go run ./cmd/forge-native

build:
	@mkdir -p build/bin
	go build -o $(BINARY) ./cmd/forge-native
	@ls -lh $(BINARY)

test:
	go test -v ./...

vet:
	go vet ./...

clean:
	rm -rf build/bin
