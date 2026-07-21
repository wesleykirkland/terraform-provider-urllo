default: fmt lint install generate

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	golangci-lint run

generate:
	cd tools; go generate ./...

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

testacc:
	TF_ACC=1 go test -v -cover -timeout 120m ./...

# cover runs the full suite (including the mock-backed acceptance tests, which
# need no credentials) and fails if total coverage drops below COVER_MIN. The
# remaining uncovered statements are unreachable defensive guards documented in
# COVERAGE.md.
COVER_MIN ?= 97.0
cover:
	TF_ACC=1 go test -timeout 30m -coverprofile=coverage.out ./...
	@total=$$(go tool cover -func=coverage.out | awk '/^total:/ {print substr($$3, 1, length($$3)-1)}'); \
	echo "total coverage: $$total% (min $(COVER_MIN)%)"; \
	awk "BEGIN { exit !($$total >= $(COVER_MIN)) }" || { echo "coverage below $(COVER_MIN)%"; exit 1; }

.PHONY: fmt lint test testacc cover build install generate
