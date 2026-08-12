#!/usr/bin/env python3
# Copyright (c) Wesley Kirkland-Daily
# SPDX-License-Identifier: MPL-2.0

"""Fails if any line added on this branch (relative to main) in a non-test
Go file isn't covered by the tests exercised in coverage.out.

This approximates SonarCloud's "Coverage on New Code" quality gate locally,
in `make cover-new` / the pre-commit hook, so a gap like rule_types.go
shipping at 30% new-code coverage gets caught before it ever reaches a
Sonar report. See AGENTS.md for the coverage policy this enforces: a
repo-wide floor (COVERAGE.md, currently 97%, with a few documented
unreachable guards) versus this stricter, near-zero-tolerance floor on code
that's actually new.

Only line-range granularity is available from a `go test -coverprofile`
profile (each block covers a contiguous statement range, not a single
line), so a changed line inside a block is treated as covered/uncovered
based on that whole block's hit count -- the same approximation
`go tool cover -html` makes when highlighting lines red/green.

Run directly with (after generating coverage.out via `make cover-new`,
`make cover`, or `go test -coverprofile=coverage.out ./...`):
    python3 scripts/check_new_code_coverage.py

NEW_CODE_COVER_MIN (env var, default 100.0) overrides the floor, mirroring
GNUmakefile's COVER_MIN pattern for the repo-wide gate.
"""

from __future__ import annotations

import os
import re
import subprocess
import sys
from pathlib import Path

COVERAGE_PROFILE = "coverage.out"
BASE_BRANCH = "main"
NEW_CODE_COVER_MIN = float(os.environ.get("NEW_CODE_COVER_MIN", "100.0"))

HUNK_RE = re.compile(r"^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@")
PROFILE_LINE_RE = re.compile(r"^(\S+):(\d+)\.\d+,(\d+)\.\d+ \d+ (\d+)$")


def repo_root() -> Path:
    out = subprocess.run(
        ["git", "rev-parse", "--show-toplevel"],
        capture_output=True,
        text=True,
        check=True,
    )
    return Path(out.stdout.strip())


def merge_base(root: Path, branch: str = BASE_BRANCH) -> str | None:
    """Returns the merge-base of HEAD and `branch`, or None if `branch`
    isn't resolvable locally (e.g. a shallow clone, or checking out the
    base branch itself)."""
    result = subprocess.run(
        ["git", "merge-base", branch, "HEAD"],
        cwd=root,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        return None
    return result.stdout.strip()


def diff_against(root: Path, base: str) -> str:
    # --cached: diffs against the index (what's about to be committed), not
    # just prior commits -- so this catches gaps in the change being
    # committed right now, not only ones already on the branch.
    # --unified=0: only genuinely added/removed lines appear, no context
    # lines to filter back out.
    result = subprocess.run(
        ["git", "diff", "--unified=0", base, "--cached", "--", "*.go", ":(exclude)*_test.go"],
        cwd=root,
        capture_output=True,
        text=True,
        check=True,
    )
    return result.stdout


def parse_added_lines(diff_text: str) -> dict[str, set[int]]:
    """Maps each changed file's path (e.g. "internal/provider/x.go") to the
    set of line numbers added in its new version."""
    added: dict[str, set[int]] = {}
    current_file: str | None = None
    next_line = 0
    for line in diff_text.splitlines():
        if line.startswith("+++ "):
            path = line[4:].strip()
            current_file = None if path == "/dev/null" else path.split("/", 1)[1]
            continue
        if line.startswith("@@"):
            m = HUNK_RE.match(line)
            if m:
                next_line = int(m.group(1))
            continue
        if current_file is None:
            continue
        if line.startswith("+"):
            added.setdefault(current_file, set()).add(next_line)
            next_line += 1
    return added


def parse_coverage_profile(profile_text: str) -> dict[str, list[tuple[int, int, int]]]:
    """Maps each covered file's module-qualified path to a list of
    (start_line, end_line, hit_count) statement blocks."""
    blocks: dict[str, list[tuple[int, int, int]]] = {}
    for line in profile_text.splitlines():
        m = PROFILE_LINE_RE.match(line)
        if not m:
            continue  # the "mode: ..." header, or a blank trailing line
        file_path, start, end, count = m.group(1), int(m.group(2)), int(m.group(3)), int(m.group(4))
        blocks.setdefault(file_path, []).append((start, end, count))
    return blocks


def find_uncovered(
    added: dict[str, set[int]],
    profile: dict[str, list[tuple[int, int, int]]],
) -> dict[str, list[int]]:
    """Cross-references added lines against the coverage profile. A line is
    reported only when it falls inside a known statement block with a zero
    hit count; lines outside any block (blank lines, comments, braces) are
    assumed non-executable and skipped, as are files with no blocks at all
    (not part of `go test ./...`'s package graph, e.g. tools/tools.go)."""
    uncovered: dict[str, list[int]] = {}
    for changed_file, lines in added.items():
        file_blocks = next(
            (blocks for path, blocks in profile.items() if path.endswith(changed_file)),
            None,
        )
        if file_blocks is None:
            continue
        for ln in sorted(lines):
            if any(start <= ln <= end and count == 0 for start, end, count in file_blocks):
                uncovered.setdefault(changed_file, []).append(ln)
    return uncovered


def main() -> int:
    root = repo_root()
    profile_path = root / COVERAGE_PROFILE
    if not profile_path.exists():
        print(
            f"{COVERAGE_PROFILE} not found -- run `go test -coverprofile={COVERAGE_PROFILE} ./...` "
            "(or `make cover-new`) first.",
            file=sys.stderr,
        )
        return 1

    base = merge_base(root)
    if base is None:
        print(
            f"Could not resolve '{BASE_BRANCH}' locally -- skipping new-code coverage check. "
            f"Fetch it (e.g. `git fetch origin {BASE_BRANCH}:{BASE_BRANCH}`) to enable this check.",
            file=sys.stderr,
        )
        return 0

    added = parse_added_lines(diff_against(root, base))
    profile = parse_coverage_profile(profile_path.read_text(encoding="utf-8"))
    uncovered = find_uncovered(added, profile)

    total_added = sum(len(lines) for lines in added.values())
    total_uncovered = sum(len(lines) for lines in uncovered.values())
    percent = 100.0 if total_added == 0 else 100.0 * (total_added - total_uncovered) / total_added

    if percent >= NEW_CODE_COVER_MIN:
        print(f"New-code coverage: {percent:.1f}% (min {NEW_CODE_COVER_MIN}%), OK.")
        return 0

    print(
        f"New-code coverage: {percent:.1f}% is below the {NEW_CODE_COVER_MIN}% floor "
        f"({total_uncovered} uncovered line(s) vs {BASE_BRANCH}):",
        file=sys.stderr,
    )
    for file, lines in sorted(uncovered.items()):
        print(f"  {file}: line(s) {', '.join(str(ln) for ln in lines)}", file=sys.stderr)
    print(
        "Add tests covering the lines above, or -- if it's a genuinely unreachable "
        "defensive guard -- document it in COVERAGE.md the way the existing exceptions are.",
        file=sys.stderr,
    )
    return 1


if __name__ == "__main__":  # pragma: no cover
    sys.exit(main())
