#!/usr/bin/env python3
# Copyright (c) Wesley Kirkland-Daily
# SPDX-License-Identifier: MPL-2.0

"""Runs this repo's Python unit tests (currently just test_check_docs.py).

If the `coverage` CLI is on PATH, also writes a Cobertura-format XML report
to <repo root>/coverage.xml -- the `coverage xml` command's output is a
Cobertura-schema report, the same family of format Go's `coverage.out` plays
in for this repo's SonarCloud/Codecov integration (see
sonar.python.coverage.reportPaths in sonar-project.properties). Coverage is
an optional nicety, not a hard dependency: scripts/ stays pure standard
library otherwise, so a contributor without `coverage` installed (via pip,
pipx, mise, etc.) still gets test results, just no report. Mirrors how
`.githooks/pre-commit` soft-skips `golangci-lint` / `ruff` when they aren't
on PATH.

Shells out to the `coverage` CLI rather than `import coverage` so this works
the same whether `coverage` came from a plain `pip install` into this
interpreter or an isolated tool install (e.g. mise's `pipx:coverage`, which
puts `coverage` on PATH in its own venv, not importable from here).
"""

from __future__ import annotations

import shutil
import subprocess
import sys
from pathlib import Path

SCRIPTS_DIR = Path(__file__).resolve().parent
ROOT = SCRIPTS_DIR.parent
COVERAGE_XML = ROOT / "coverage.xml"
TEST_PATTERN = "test_*.py"

# Overridable so CI (or a contributor) can enforce a floor once the suite is
# large enough to make one meaningful; 0 means "report only, never fail".
COVERAGE_MIN = 100.0

# Only the test files themselves are excluded -- run_python_tests.py's own
# logic is covered by scripts/test_run_python_tests.py (mocking subprocess.run
# rather than actually spawning nested subprocesses).
OMIT = str(SCRIPTS_DIR / TEST_PATTERN)


def run_plain() -> int:
    result = subprocess.run(
        [sys.executable, "-m", "unittest", "discover", "-s", str(SCRIPTS_DIR), "-p", TEST_PATTERN, "-v"],
        cwd=ROOT,
    )
    return result.returncode


def run_with_coverage(coverage_bin: str) -> int:
    test_result = subprocess.run(
        [
            coverage_bin,
            "run",
            f"--source={SCRIPTS_DIR}",
            f"--omit={OMIT}",
            "-m",
            "unittest",
            "discover",
            "-s",
            str(SCRIPTS_DIR),
            "-p",
            TEST_PATTERN,
            "-v",
        ],
        cwd=ROOT,
    )

    xml_result = subprocess.run([coverage_bin, "xml", "-o", str(COVERAGE_XML)], cwd=ROOT)

    report_cmd = [coverage_bin, "report", "-m"]
    if COVERAGE_MIN > 0:
        report_cmd.append(f"--fail-under={COVERAGE_MIN}")
    report_result = subprocess.run(report_cmd, cwd=ROOT)

    if test_result.returncode != 0:
        return test_result.returncode
    if xml_result.returncode != 0:
        return xml_result.returncode
    return report_result.returncode


def main() -> int:
    coverage_bin = shutil.which("coverage")
    if not coverage_bin:
        print(
            "coverage not found on PATH -- running tests without a coverage report "
            "(pip install coverage, or mise install, for one).",
            file=sys.stderr,
        )
        return run_plain()
    return run_with_coverage(coverage_bin)


if __name__ == "__main__":
    sys.exit(main())  # pragma: no cover
