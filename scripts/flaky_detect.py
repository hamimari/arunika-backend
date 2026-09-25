#!/usr/bin/env python3
"""Flaky-test detection over repeated runs of one unchanged commit (design D7).

    flaky_detect.py run1.xml run2.xml run3.xml ...

Each argument is a JUnit report from a full run of the same commit. A test that
passed in some runs and failed in others is flaky: it is listed and the script
exits 1. A test that failed in every run is a plain failure and also exits 1,
but is reported separately so nobody mistakes it for flakiness.
"""
import sys
import xml.etree.ElementTree as ET
from collections import defaultdict


def outcomes(path):
    result = {}
    for case in ET.parse(path).getroot().iter("testcase"):
        name = f"{case.get('classname', '')}::{case.get('name', '')}"
        if case.find("skipped") is not None:
            continue
        failed = case.find("failure") is not None or case.find("error") is not None
        result[name] = not failed
    return result


def main(paths):
    if len(paths) < 2:
        print("need at least two runs to compare", file=sys.stderr)
        return 2
    seen = defaultdict(list)
    for path in paths:
        for name, ok in outcomes(path).items():
            seen[name].append(ok)

    flaky = sorted(n for n, r in seen.items() if any(r) and not all(r))
    broken = sorted(n for n, r in seen.items() if not any(r))

    print(f"{len(seen)} tests across {len(paths)} runs")
    if flaky:
        print(f"\nFLAKY ({len(flaky)}) — passed and failed on the same commit:")
        for name in flaky:
            passes = sum(seen[name])
            print(f"  {name}  passed {passes}/{len(seen[name])}")
    if broken:
        print(f"\nFAILING in every run ({len(broken)}):")
        for name in broken:
            print(f"  {name}")
    if not flaky and not broken:
        print("no flaky tests detected")
    return 1 if flaky or broken else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
