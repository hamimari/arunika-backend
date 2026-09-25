# Local and CI use the same commands. See README "Running Tests".
.PHONY: lint fmt test-fast test-all test-e2e e2e-hold coverage-baseline build

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

## test-e2e: cross-system flows against the real docker-compose stack (needs Docker;
## builds ../arunika-backoffice too, so that repo must sit next to this one).
test-e2e:
	go test -tags e2e ./tests/e2e/ -count=1 -timeout 20m -v

## e2e-hold: bring the E2E stack up and hold it (Ctrl-C tears it down) so the
## backoffice Playwright specs can run against it: api :8090, backoffice :3010.
e2e-hold:
	E2E_HOLD=1 go test -tags e2e ./tests/e2e/ -run TestStackHold -count=1 -timeout 120m -v

## coverage-baseline: re-record the ratchet baseline after intentional changes.
coverage-baseline:
	go test ./... -coverpkg=./... -coverprofile=cover.out
	python3 scripts/coverage-ratchet.py --write
