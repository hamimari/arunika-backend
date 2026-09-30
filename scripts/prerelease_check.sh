#!/usr/bin/env bash
# Pre-release gate for arunika-backend. Spec and severity policy:
# arunika_app/openspec/changes/add-prerelease-security-gate
#
# Usage: scripts/prerelease_check.sh [--allow-missing] [--skip-load] [--write-baselines]
#   --allow-missing    a missing tool is a warning instead of a failure
#   --skip-load        skip the k6 load test (needs the API running at BACKEND_URL)
#   --write-baselines  re-record bench-baseline.txt after an intentional change
set -uo pipefail
cd "$(dirname "$0")/.."

BACKEND_URL="${BACKEND_URL:-http://localhost:8080}"
REPORT_DIR=prerelease-reports
BENCH_BASELINE=bench-baseline.txt
ALLOW_MISSING=false SKIP_LOAD=false WRITE_BASELINES=false
for arg in "$@"; do
  case "$arg" in
    --allow-missing) ALLOW_MISSING=true ;;
    --skip-load) SKIP_LOAD=true ;;
    --write-baselines) WRITE_BASELINES=true ;;
    *) echo "unknown option: $arg"; sed -n 5,8p "$0"; exit 2 ;;
  esac
done

# go install puts gosec/govulncheck here, which non-login shells often lack.
PATH="$PATH:$(go env GOPATH)/bin"
mkdir -p "$REPORT_DIR"

RED='\033[0;31m' GREEN='\033[0;32m' YELLOW='\033[1;33m' NC='\033[0m'
FAILED=() SKIPPED=()
section() { echo -e "\n${YELLOW}==> $1${NC}"; }
ok()      { echo -e "${GREEN}  ✓ $1${NC}"; }
fail()    { echo -e "${RED}  ✗ $1${NC}"; FAILED+=("$1"); }
skip()    { echo -e "  - skipped: $1"; SKIPPED+=("$1"); }

# need TOOL INSTALL_HINT — succeeds when TOOL is on PATH. A missing tool fails
# the gate unless --allow-missing, so an unscanned repo never looks clean.
need() {
  command -v "$1" >/dev/null 2>&1 && return 0
  if $ALLOW_MISSING; then skip "$1 not installed ($2)"; else fail "$1 not installed — $2"; fi
  return 1
}

# check NAME REPORT CMD... — runs CMD with output to REPORT; pass/fail on exit code.
check() {
  local name="$1" report="$REPORT_DIR/$2"; shift 2
  if "$@" >"$report" 2>&1; then ok "$name"; else fail "$name — see $report"; fi
}

section "Secrets"
if need gitleaks "brew install gitleaks"; then
  check "gitleaks: git history" gitleaks-history.txt \
    gitleaks git --no-banner --redact --report-path "$REPORT_DIR/gitleaks-history.json" .
  # History covers committed files; this covers edits and new files not yet committed.
  changed=$(git ls-files -mo --exclude-standard | grep -v "^$REPORT_DIR/")
  if [ -n "$changed" ]; then
    leaks=0
    while IFS= read -r f; do
      [ -f "$f" ] || continue
      gitleaks dir --no-banner --redact "$f" >>"$REPORT_DIR/gitleaks-worktree.txt" 2>&1 || leaks=1
    done <<<"$changed"
    if [ "$leaks" = 0 ]; then ok "gitleaks: uncommitted changes"
    else fail "gitleaks: uncommitted changes — see $REPORT_DIR/gitleaks-worktree.txt"; fi
  else
    ok "gitleaks: uncommitted changes (none)"
  fi
fi

section "Static analysis"
if need gosec "go install github.com/securego/gosec/v2/cmd/gosec@latest"; then
  # tests/ is test-only code (fixtures, fakes) that never ships.
  check "gosec (medium and above)" gosec.txt gosec -quiet -severity medium -exclude-dir=tests ./...
fi
if need govulncheck "go install golang.org/x/vuln/cmd/govulncheck@latest"; then
  check "govulncheck: no reachable vulnerabilities" govulncheck.txt govulncheck ./...
fi
if need golangci-lint "brew install golangci-lint"; then
  check "make lint (gofmt + golangci-lint)" lint.txt make lint
fi
check "no TODO/FIXME in auth, payment or entitlement code" todo.txt \
  bash -c "! grep -rnE 'TODO|FIXME' --include='*.go' . | grep -iE '^[^:]*(auth|pay|purchase|billing|entitle|order|token|consent|security)'"

section "Tests"
check "go test -race" go-test-race.txt go test -race -count=1 ./...

section "Benchmarks"
if ! grep -rqE '^func Benchmark' --include='*_test.go' .; then
  skip "benchmarks: none defined in the repo"
else
  go test -run '^$' -bench . -benchmem ./... >"$REPORT_DIR/bench.txt" 2>&1 || fail "benchmarks failed to run — see $REPORT_DIR/bench.txt"
  if $WRITE_BASELINES || [ ! -f "$BENCH_BASELINE" ]; then
    cp "$REPORT_DIR/bench.txt" "$BENCH_BASELINE"; ok "benchmarks: baseline recorded in $BENCH_BASELINE"
  else
    # ns/op regressions above 20% against the committed baseline.
    regressions=$(awk '
      FNR==NR && /^Benchmark/ { base[$1]=$3; next }
      /^Benchmark/ && ($1 in base) && base[$1] > 0 && $3 > base[$1]*1.2 {
        printf "%s %s -> %s ns/op\n", $1, base[$1], $3 }' "$BENCH_BASELINE" "$REPORT_DIR/bench.txt")
    if [ -z "$regressions" ]; then ok "benchmarks: within 20% of baseline"
    else echo "$regressions" >"$REPORT_DIR/bench-regressions.txt"; fail "benchmarks regressed >20% — see $REPORT_DIR/bench-regressions.txt"; fi
  fi
fi

section "Load test"
if $SKIP_LOAD; then
  skip "k6 load test (--skip-load)"
elif ! curl -fsS -o /dev/null "$BACKEND_URL/health"; then
  skip "k6 load test: API not reachable at $BACKEND_URL (start it, or pass --skip-load)"
elif need k6 "brew install k6"; then
  check "k6: p95 < 500ms and error rate < 1% at 20 VUs" k6.txt \
    env TARGET_URL="$BACKEND_URL" k6 run scripts/load-test.js
fi

section "Summary"
echo "Failures: ${#FAILED[@]}   Skipped: ${#SKIPPED[@]}   Reports: $REPORT_DIR/"
for f in ${FAILED[@]+"${FAILED[@]}"}; do echo -e "  ${RED}✗${NC} $f"; done
for s in ${SKIPPED[@]+"${SKIPPED[@]}"}; do echo "  - $s"; done
echo "Triage every finding in arunika_app/docs/prerelease-triage.md before release."
[ "${#FAILED[@]}" -eq 0 ]
