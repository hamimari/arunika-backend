#!/usr/bin/env python3
"""Coverage ratchet — compares per-package coverage against coverage-baseline.json.

REPORT-ONLY by default: it prints drops and exits 0. Set RATCHET_ENFORCE=1 to
make a drop fail the build.

Coverage is measured with -coverpkg=./... so that a package is credited for
code exercised by tests living anywhere in the module. Without it, the
database tests under tests/db would count for nothing against the `models`
package they exist to cover, and `models` would keep reporting 0%.

Usage:
    go test ./... -coverpkg=./... -coverprofile=cover.out
    python3 scripts/coverage-ratchet.py [--write]
"""

# macOS ships Python 3.9, where builtin generics in annotations are not
# evaluated the same way; deferring annotations keeps this runnable there.
from __future__ import annotations

import json
import os
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
PROFILE = ROOT / "cover.out"
BASELINE = ROOT / "coverage-baseline.json"
ENFORCE = os.environ.get("RATCHET_ENFORCE") == "1"
WRITE = "--write" in sys.argv

# "import/path/file.go:12.34,15.6 3 1" -> (import/path, statements, count)
LINE = re.compile(r"^(?P<path>.+)/[^/]+\.go:\d+\.\d+,\d+\.\d+ (?P<stmts>\d+) (?P<count>\d+)$")


def module_path() -> str:
    for line in (ROOT / "go.mod").read_text().splitlines():
        if line.startswith("module "):
            return line.split(None, 1)[1].strip()
    sys.exit("could not determine module path from go.mod")


def all_packages(mod: str) -> list[str]:
    out = subprocess.run(
        ["go", "list", "./..."], cwd=ROOT, capture_output=True, text=True, check=True
    ).stdout
    pkgs = []
    for p in out.split():
        rel = p[len(mod):].lstrip("/")
        pkgs.append(rel or ".")
    return pkgs


def measured() -> dict[str, tuple[int, int]]:
    """Per-package (covered_statements, total_statements) from the profile.

    Blocks are deduplicated by their exact source span before being summed.
    Under -coverpkg=./... every test binary emits a block for every package,
    so the same statements appear once per binary; summing them naively
    multiplies the denominator by the number of packages and reports coverage
    roughly an order of magnitude too low. A block counts as covered if any
    binary executed it, which is what "is this code exercised by the test
    suite" means.
    """
    if not PROFILE.exists():
        sys.exit(f"No coverage profile at {PROFILE.relative_to(ROOT)}. "
                 f"Run: go test ./... -coverpkg=./... -coverprofile=cover.out")

    # span -> (package, statements, covered)
    blocks: dict[str, tuple[str, int, bool]] = {}
    for raw in PROFILE.read_text().splitlines():
        if raw.startswith("mode:") or not raw.strip():
            continue
        m = LINE.match(raw)
        if not m:
            continue
        span = raw.rsplit(" ", 2)[0]  # "path/file.go:12.34,15.6"
        stmts = int(m.group("stmts"))
        hit = int(m.group("count")) > 0
        prev = blocks.get(span)
        blocks[span] = (m.group("path"), stmts, hit or (prev[2] if prev else False))

    acc: dict[str, list[int]] = {}
    for pkg, stmts, hit in blocks.values():
        entry = acc.setdefault(pkg, [0, 0])
        entry[0] += stmts if hit else 0
        entry[1] += stmts
    return {k: (v[0], v[1]) for k, v in acc.items()}


def main() -> int:
    mod = module_path()
    raw = measured()

    # Re-key the profile's absolute import paths to module-relative names.
    by_pkg: dict[str, tuple[int, int]] = {}
    for pkg, counts in raw.items():
        rel = pkg[len(mod):].lstrip("/") if pkg.startswith(mod) else pkg
        by_pkg[rel or "."] = counts

    current: dict[str, float] = {}
    for pkg in all_packages(mod):
        # The tests/ tree is harness code — fixtures, builders, the fake Play
        # server. Ratcheting coverage *of the helpers* measures nothing about
        # the product and churns whenever an unused builder option is added.
        if pkg == "tests" or pkg.startswith("tests/"):
            continue
        covered, total = by_pkg.get(pkg, (0, 0))
        # No statements measured => either no test files, or nothing to cover.
        current[pkg] = round(covered / total * 100, 1) if total else 0.0

    if WRITE or not BASELINE.exists():
        BASELINE.write_text(json.dumps(current, indent=2, sort_keys=True) + "\n")
        print(f"Wrote baseline to {BASELINE.relative_to(ROOT)}:")
        for k, v in sorted(current.items()):
            print(f"  {k:<24} {v}%")
        return 0

    baseline = json.loads(BASELINE.read_text())
    drops = []
    for pkg, pct in sorted(current.items()):
        base = baseline.get(pkg)
        if base is None:
            print(f"  {pkg:<24} {pct}%  (new — not yet in baseline)")
            continue
        # 0.1pp of slack absorbs rounding noise.
        if pct < base - 0.1:
            drops.append((pkg, base, pct))
            print(f"  {pkg:<24} {pct}%  <- down from {base}%")
        else:
            print(f"  {pkg:<24} {pct}%")

    if not drops:
        print("\nCoverage ratchet: no regressions.")
        return 0

    print(f"\nCoverage ratchet: {len(drops)} package(s) dropped.")
    if not ENFORCE:
        print("Report-only mode — not failing the build. Set RATCHET_ENFORCE=1 to enforce.")
        return 0
    return 1


if __name__ == "__main__":
    sys.exit(main())
