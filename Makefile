BINARY := lazygaze

.PHONY: build test clean

build:
	go build -o $(BINARY) .

test:
	go test ./...

clean:
	rm -f $(BINARY)
