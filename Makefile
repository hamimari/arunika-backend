# Local and CI use the same commands. See README "Running Tests".
.PHONY: lint fmt test-fast test-all coverage-baseline build

fmt:
	gofmt -w .

lint:
	@unformatted=$$(gofmt -l . | grep -v '^arunika_backend$$' || true); \
	if [ -n "$$unformatted" ]; then \
		echo "These files are not gofmt'd:"; echo "$$unformatted"; exit 1; \
	fi
	golangci-lint run

build:
	go build ./...

## test-fast: the inner-loop command. No race detector, no coverage.
test-fast:
	go test ./...

## test-all: everything CI runs on a pull request.
test-all: lint build
	mkdir -p reports
	gotestsum --junitfile reports/junit.xml --format testname \
		-- -race -coverpkg=./... -coverprofile=cover.out ./...
	python3 scripts/coverage-ratchet.py

## coverage-baseline: re-record the ratchet baseline after intentional changes.
coverage-baseline:
	go test ./... -coverpkg=./... -coverprofile=cover.out
	python3 scripts/coverage-ratchet.py --write
