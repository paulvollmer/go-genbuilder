all: test build

.PHONY: test
test:
	go test ./...

.PHONY: test-cover
test-cover:
	go test -cover ./...

bin/golangci-lint:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.59.1
lint: bin/golangci-lint
	@ ./bin/golangci-lint run -c ./.golangci.yaml ./...
lint-fix: bin/golangci-lint
	@ ./bin/golangci-lint run -c ./.golangci.yaml --fix ./...

.PHONY: build
build:
	go build

.PHONY: example-gen
example-gen: build
	cd example && go generate -tags=example
