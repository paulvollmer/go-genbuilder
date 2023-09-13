all: test build

test:
	go test ./...

build:
	go build

example-gen: build
	cd example && go generate -tags=example
