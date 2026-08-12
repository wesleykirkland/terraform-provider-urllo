#!/usr/bin/env python3
# Copyright (c) Wesley Kirkland-Daily
# SPDX-License-Identifier: MPL-2.0

"""Fails if any line added on this branch (relative to main) in a non-test
Go file isn't covered by the tests exercised in coverage.out -- unless it's
explicitly marked as an unreachable defensive guard.

This approximates SonarCloud's "Coverage on New Code" quality gate locally,
in `make cover-new` / the pre-commit hook, so a gap like rule_types.go
shipping at 30% new-code coverage gets caught before it ever reaches a
Sonar report. See AGENTS.md for the coverage policy this enforces.

Not all Go statements in this codebase CAN be covered: COVERAGE.md documents
a handful of defensive guards (e.g. a schema-decode failure the framework
can't actually produce) that are deliberately kept for robustness even
though no test can reach them. A blind "100% of added lines" gate would
either have to ignore that reality (weakening the check for everything else)
or produce false failures on legitimate, already-accepted patterns. Instead:
a line inside an uncovered statement block is exempted from this check only
if that block's source contains a trailing `coverage:ignore` comment (e.g.
`// coverage:ignore: Read always supplies an ID, so this is unreachable`) --
explicit, grep-able, and reviewed like any other line in the diff, rather
than a percentage threshold hiding the same gap.

Only line-range granularity is available from a `go test -coverprofile`
profile (each block covers a contiguous statement range, not a single
line), so a changed line inside a block is treated as covered/uncovered
based on that whole block's hit count -- the same approximation
`go tool cover -html` makes when highlighting lines red/green. The
`coverage:ignore` marker is likewise resolved per-block: it can be placed on
any line within the uncovered block, and it exempts the whole block.

Run directly with (after generating coverage.out via `make cover-new`,
`make cover`, or `go test -coverprofile=coverage.out ./...`):
    python3 scripts/check_new_code_coverage.py

COVER_MIN (env var, default 97.0) overrides the floor -- the same variable
GNUmakefile's `cover` target uses for the repo-wide gate. Both gates share
one name/default; override per-invocation (e.g. `make cover-new
COVER_MIN=100`) for a stricter local check on new code specifically.
"""

from __future__ import annotations

import os
import re
import subprocess
import sys
from pathlib import Path

COVERAGE_PROFILE = "coverage.out"
BASE_BRANCH = "main"
COVER_MIN = float(os.environ.get("COVER_MIN", "97.0"))
IGNORE_MARKER = "coverage:ignore"

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


def block_is_ignored(source_lines: list[str], start: int, end: int) -> bool:
    """Whether any line in the 1-indexed [start, end] range of `source_lines`
    (the file's current, on-disk content) carries an IGNORE_MARKER comment."""
    return any(IGNORE_MARKER in ln for ln in source_lines[start - 1 : end])


def blocks_for_file(
    profile: dict[str, list[tuple[int, int, int]]],
    changed_file: str,
) -> list[tuple[int, int, int]] | None:
    """Looks up `changed_file` (a repo-relative path) in `profile` (keyed by
    module-qualified path), matching on suffix."""
    return next(
        (blocks for path, blocks in profile.items() if path.endswith(changed_file)),
        None,
    )


def line_is_uncovered(
    ln: int,
    file_blocks: list[tuple[int, int, int]],
    file_source: list[str] | None,
) -> bool:
    """Whether `ln` falls inside a zero-hit statement block that isn't
    exempted by a `coverage:ignore` marker anywhere in that block."""
    for start, end, count in file_blocks:
        if not (start <= ln <= end and count == 0):
            continue
        return not (file_source and block_is_ignored(file_source, start, end))
    return False


def find_uncovered(
    added: dict[str, set[int]],
    profile: dict[str, list[tuple[int, int, int]]],
    sources: dict[str, list[str]] | None = None,
) -> dict[str, list[int]]:
    """Cross-references added lines against the coverage profile. A line is
    reported only when it falls inside a known statement block with a zero
    hit count and no `coverage:ignore` marker anywhere in that block; lines
    outside any block (blank lines, comments, braces) are assumed
    non-executable and skipped, as are files with no blocks at all (not part
    of `go test ./...`'s package graph, e.g. tools/tools.go).

    `sources` maps each changed file to its current source lines, used only
    to resolve the ignore marker; omit it (or a file's entry) to disable
    marker resolution for that file."""
    sources = sources or {}
    uncovered: dict[str, list[int]] = {}
    for changed_file, lines in added.items():
        file_blocks = blocks_for_file(profile, changed_file)
        if file_blocks is None:
            continue
        file_source = sources.get(changed_file)
        flagged = [ln for ln in sorted(lines) if line_is_uncovered(ln, file_blocks, file_source)]
        if flagged:
            uncovered[changed_file] = flagged
    return uncovered


def load_sources(root: Path, files: list[str]) -> dict[str, list[str]]:
    """Reads the current (working-tree) source lines for each changed file,
    used to resolve `coverage:ignore` markers. A file missing on disk (e.g.
    deleted since staging) is simply omitted."""
    sources: dict[str, list[str]] = {}
    for f in files:
        path = root / f
        if path.is_file():
            sources[f] = path.read_text(encoding="utf-8").splitlines()
    return sources


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
    sources = load_sources(root, list(added))
    uncovered = find_uncovered(added, profile, sources)

    total_added = sum(len(lines) for lines in added.values())
    total_uncovered = sum(len(lines) for lines in uncovered.values())
    percent = 100.0 if total_added == 0 else 100.0 * (total_added - total_uncovered) / total_added

    if percent >= COVER_MIN:
        print(f"New-code coverage: {percent:.1f}% (min {COVER_MIN}%), OK.")
        return 0

    print(
        f"New-code coverage: {percent:.1f}% is below the {COVER_MIN}% floor "
        f"({total_uncovered} uncovered line(s) vs {BASE_BRANCH}):",
        file=sys.stderr,
    )
    for file, lines in sorted(uncovered.items()):
        print(f"  {file}: line(s) {', '.join(str(ln) for ln in lines)}", file=sys.stderr)
    print(
        "Add tests covering the lines above, or -- if it's a genuinely unreachable defensive "
        f"guard -- mark it with a trailing `// {IGNORE_MARKER}: <reason>` comment (see "
        "host_resource.go's Read nil-guard for an example) and note it in COVERAGE.md.",
        file=sys.stderr,
    )
    return 1


if __name__ == "__main__":  # pragma: no cover
    sys.exit(main())
