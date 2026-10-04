#!/usr/bin/env python3
"""Build the WHO LMS file and golden fixture used by the growth engine.

Downloads the WHO Child Growth Standards 2006 expanded daily tables
(length/height-for-age and weight-for-age, boys and girls, days 0-1856) and
writes:

  growth/data/who2006_lms.csv     indicator,sex,age_days,l,m,s
  growth/testdata/golden.csv      indicator,sex,age_days,position,value,expected_z

The golden rows are WHO's own SD columns (SD4neg..SD4) at monthly ages, so
each value must classify to exactly that z-score (+-0.01). WHO extends the
weight SD3/SD4 lines linearly, which is what the restricted LMS tail rule
inverts, so they hold for both indicators.

The app ships a byte-identical copy of both files; see the SHA-256 constant in
growth/lms_test.go and the app's test/core/growth/growth_engine_test.dart.

Usage: pip install openpyxl && python3 scripts/build_who_lms.py
"""
import io
import os
import urllib.request

import openpyxl

BASE = "https://cdn.who.int/media/docs/default-source/child-growth/child-growth-standards/indicators"
SOURCES = {
    ("lhfa", "male"): f"{BASE}/length-height-for-age/expandable-tables/lhfa-boys-zscore-expanded-tables.xlsx",
    ("lhfa", "female"): f"{BASE}/length-height-for-age/expandable-tables/lhfa-girls-zscore-expanded-tables.xlsx",
    ("wfa", "male"): f"{BASE}/weight-for-age/expanded-tables/wfa-boys-zscore-expanded-tables.xlsx",
    ("wfa", "female"): f"{BASE}/weight-for-age/expanded-tables/wfa-girls-zscore-expanded-tables.xlsx",
}
SD_COLUMNS = ["SD4neg", "SD3neg", "SD2neg", "SD1neg", "SD0", "SD1", "SD2", "SD3", "SD4"]
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def fmt(v):
    # Keep the table's own precision; avoid float noise like 49.884200000001.
    return repr(float(v)).rstrip("0").rstrip(".") if isinstance(v, float) else str(v)


def load(url):
    req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
    with urllib.request.urlopen(req) as r:
        ws = openpyxl.load_workbook(io.BytesIO(r.read()), read_only=True).active
    rows = list(ws.iter_rows(values_only=True))
    header = [str(h) for h in rows[0]]
    return [dict(zip(header, row)) for row in rows[1:] if row[0] is not None]


def main():
    lms, golden = [], []
    for (indicator, sex), url in SOURCES.items():
        rows = load(url)
        assert len(rows) == 1857, f"{indicator}/{sex}: expected days 0-1856, got {len(rows)}"
        for row in rows:
            lms.append(f"{indicator},{sex},{row['Day']},{fmt(row['L'])},{fmt(row['M'])},{fmt(row['S'])}")
        for month in range(0, 61):
            day = round(month * 30.4375)
            row = rows[day]
            position = "recumbent" if day < 731 else "standing"
            for k, col in zip(range(-4, 5), SD_COLUMNS):
                golden.append(f"{indicator},{sex},{day},{position},{fmt(row[col])},{k}")

    data_dir = os.path.join(ROOT, "growth", "data")
    test_dir = os.path.join(ROOT, "growth", "testdata")
    os.makedirs(data_dir, exist_ok=True)
    os.makedirs(test_dir, exist_ok=True)
    with open(os.path.join(data_dir, "who2006_lms.csv"), "w", newline="\n") as f:
        f.write("indicator,sex,age_days,l,m,s\n" + "\n".join(lms) + "\n")
    with open(os.path.join(test_dir, "golden.csv"), "w", newline="\n") as f:
        f.write("indicator,sex,age_days,position,value,expected_z\n" + "\n".join(golden) + "\n")
    print(f"wrote {len(lms)} LMS rows and {len(golden)} golden rows")


if __name__ == "__main__":
    main()
