.PHONY: build test lint clean

build:
	go build ./...

test:
	go test ./... -race -count=1

lint:
	golangci-lint run ./...

clean:
	rm -f wallet-demo

all: lint test build
