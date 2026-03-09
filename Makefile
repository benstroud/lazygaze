BINARY := lazygaze
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test clean

build:
	go build -ldflags "-X github.com/benstroud/lazygaze/cmd.Version=$(VERSION)" -o $(BINARY) .

test:
	go test ./...

clean:
	rm -f $(BINARY)
