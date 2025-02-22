.DEFAULT_GOAL = check

# renovate: datasource=github-releases depName=golangci/golangci-lint
GOLANGCI_VERSION ?= v1.63.4
TEST_ARGS=-timeout 5s -coverpkg=github.com/ladzaretti/migrate

bin/golangci-lint: bin/golangci-lint-${GOLANGCI_VERSION}
	@ln -sf golangci-lint-${GOLANGCI_VERSION} bin/golangci-lint

bin/golangci-lint-${GOLANGCI_VERSION}:
	@mkdir -p bin
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
    	| sh -s -- -b ./bin  $(GOLANGCI_VERSION)
	@mv bin/golangci-lint "$@"

.PHONY: clean
clean:
	go clean -testcache
	rm -rf bin/ coverage/

.PHONY: test
test:
	go test $(TEST_ARGS) ./migrate_test/

.PHONY: cover
cover:
	@mkdir -p coverage
	go test $(TEST_ARGS) ./migrate_test/ -coverprofile coverage/cover.out

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
