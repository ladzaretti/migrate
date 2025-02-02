.DEFAULT_GOAL = bin/klt

GOLANGCI_VERSION ?= v1.63.4
TEST_TIMEOUT = 5s

.PHONY: bin/klt
bin/klt:
	go build -o "$@" ./cmd/klt

bin/golangci-lint: bin/golangci-lint-${GOLANGCI_VERSION}
	@ln -sf golangci-lint-${GOLANGCI_VERSION} bin/golangci-lint

bin/golangci-lint-${GOLANGCI_VERSION}:
	@mkdir -p bin
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
    	| sh -s -- -b ./bin ${GOLANGCI_VERSION}
	@mv bin/golangci-lint "$@"

.PHONY: run
run:
	./bin/klt

.PHONY: clean
clean:
	go clean -testcache
	rm -rf bin/
	rm -rf coverage/

.PHONY: test
test:
	go test -timeout ${TEST_TIMEOUT} -cover ./...

.PHONY: cover
cover:
	@mkdir -p coverage
	go test -timeout ${TEST_TIMEOUT} ./... -coverprofile coverage/cover.out

.PHONY: coverage-html
coverage-html: cover
	go tool cover -html=coverage/cover.out -o coverage/index.html

.PHONY: lint
lint: | bin/golangci-lint ## Run linter
	bin/golangci-lint run

.PHONY: fix
fix: | bin/golangci-lint ## Fix lint violations
	bin/golangci-lint run --fix

.PHONY: check
check: lint test ## Run tests and linters
