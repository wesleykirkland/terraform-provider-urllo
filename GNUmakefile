# Overridable for contributors whose Python 3 binary isn't named `python3`
# (e.g. some Windows setups only have `python` on PATH): `make check-docs
# PYTHON=python`.
PYTHON ?= python3

default: hooks fmt lint install generate

# Points git at .githooks/ (pre-commit runs gofmt/lint/test/check-docs) so the
# hook is active without the user ever running `git config` or an install
# script themselves. This re-applies on every `make` invocation -- the same
# self-install trick husky uses via npm's "prepare" script, just triggered by
# `make` instead of `npm install` since that's this repo's equivalent entry
# point. `git config` here is a cheap, idempotent, repo-local write (like
# ~/.gitconfig, scoped to .git/config in this clone), not a destructive one.
HOOKS_DIR := .githooks

hooks:
	@git config core.hooksPath $(HOOKS_DIR)

build: hooks
	go build -v ./...

install: build
	go install -v ./...

lint: hooks
	golangci-lint run

generate: hooks
	cd tools; go generate ./...

fmt: hooks
	gofmt -s -w -e .

vet: hooks
	go vet ./...

test: hooks vet
	go test -v -race -cover -timeout=120s -parallel=10 ./...

testacc: hooks
	TF_ACC=1 go test -v -cover -timeout 120m ./...

# Verifies README.md / terraform/README.md reference every resource, data
# source, provider config option, and example .tf file that actually exists.
# See scripts/check_docs.py for what it checks and why.
check-docs: hooks
	$(PYTHON) scripts/check_docs.py

# Lints scripts/*.py and .githooks/pre-commit (see ruff.toml). Requires ruff
# (pip install ruff) -- CI installs it, but this doesn't fail locally if it's
# missing (see .githooks/pre-commit), matching how `lint` needs golangci-lint.
lint-py: hooks
	ruff check .

# Runs scripts/test_check_docs.py, with a coverage report if the `coverage`
# package is installed (see scripts/run_python_tests.py). No hard dependency
# beyond the standard library -- coverage is optional.
test-py: hooks
	$(PYTHON) scripts/run_python_tests.py

# Warns (never fails, here or in CI -- see the "api-spec-drift" job) when the
# live Urllo OpenAPI spec has moved past the version recorded in
# API_SPEC_VERSION. Urllo doesn't publish an API changelog, so this is the
# only drift signal available. See scripts/check_api_spec_version.py.
check-api-spec: hooks
	$(PYTHON) scripts/check_api_spec_version.py

# cover runs the full suite (including the mock-backed acceptance tests, which
# need no credentials) and fails if total coverage drops below COVER_MIN. The
# remaining uncovered statements are unreachable defensive guards documented in
# COVERAGE.md.
COVER_MIN ?= 97.0
cover: hooks
	TF_ACC=1 go test -timeout 30m -coverprofile=coverage.out ./...
	@total=$$(go tool cover -func=coverage.out | awk '/^total:/ {print substr($$3, 1, length($$3)-1)}'); \
	echo "total coverage: $$total% (min $(COVER_MIN)%)"; \
	awk "BEGIN { exit !($$total >= $(COVER_MIN)) }" || { echo "coverage below $(COVER_MIN)%"; exit 1; }

# cover-new is the repo-wide floor's sibling: it fails if any line *added on
# this branch relative to main* isn't covered, catching gaps a small new file
# can hide from the aggregate COVER_MIN check above (see AGENTS.md).
# Approximates SonarCloud's "Coverage on New Code" gate locally. Shares the
# same COVER_MIN variable as `cover` above -- override per-invocation (e.g.
# `make cover-new COVER_MIN=100`) for a stricter local check.
# See scripts/check_new_code_coverage.py.
cover-new: hooks
	TF_ACC=1 go test -timeout 30m -coverprofile=coverage.out ./...
	COVER_MIN=$(COVER_MIN) $(PYTHON) scripts/check_new_code_coverage.py

.PHONY: fmt lint vet test testacc cover cover-new build install generate check-docs check-api-spec lint-py test-py hooks
