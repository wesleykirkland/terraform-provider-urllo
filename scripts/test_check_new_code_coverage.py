#!/usr/bin/env python3
# Copyright Wesley Kirkland-Daily 2026
# SPDX-License-Identifier: MPL-2.0

"""Unit tests for check_new_code_coverage.py.

The diff-parsing and coverage-matching logic is pure and tested directly
against hand-written diff/profile text. git and the filesystem are mocked
throughout -- these tests don't touch this repo's real git history or
coverage.out.
"""

from __future__ import annotations

import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parent))

import check_new_code_coverage as c  # noqa: E402

DIFF_ONE_FILE = """\
diff --git a/internal/provider/rule_types.go b/internal/provider/rule_types.go
index 1111111..2222222 100644
--- a/internal/provider/rule_types.go
+++ b/internal/provider/rule_types.go
@@ -51,0 +52,7 @@ func analyticsToObject
+	obj, d := types.ObjectValue(analyticsObjectAttrTypes, map[string]attr.Value{
+		"analytics_start_date": types.StringValue(a.AnalyticsStartDate),
+		"analytics_end_date":   types.StringValue(a.AnalyticsEndDate),
+		"requests_processed":   types.Int64Value(a.RequestsProcessed),
+	})
+	diags.Append(d...)
+	return obj
"""

DIFF_NEW_FILE = """\
diff --git a/internal/client/new.go b/internal/client/new.go
new file mode 100644
index 0000000..3333333
--- /dev/null
+++ b/internal/client/new.go
@@ -0,0 +1,2 @@
+package client
+
"""

DIFF_DELETED_FILE = """\
diff --git a/internal/client/old.go b/internal/client/old.go
deleted file mode 100644
index 4444444..0000000
--- a/internal/client/old.go
+++ /dev/null
@@ -1,2 +0,0 @@
-package client
-
"""

PROFILE_TEXT = """\
mode: set
github.com/wesleykirkland/terraform-provider-urllo/internal/provider/rule_types.go:51.93,52.14 1 1
github.com/wesleykirkland/terraform-provider-urllo/internal/provider/rule_types.go:52.14,54.3 1 0
github.com/wesleykirkland/terraform-provider-urllo/internal/provider/rule_types.go:55.2,61.12 3 0
github.com/wesleykirkland/terraform-provider-urllo/internal/provider/rule_types.go:64.96,69.35 4 1
"""


class ParseAddedLinesTest(unittest.TestCase):
    def test_extracts_added_line_numbers(self) -> None:
        added = c.parse_added_lines(DIFF_ONE_FILE)
        self.assertEqual(added, {"internal/provider/rule_types.go": set(range(52, 59))})

    def test_new_file_counts_every_added_line(self) -> None:
        added = c.parse_added_lines(DIFF_NEW_FILE)
        self.assertEqual(added, {"internal/client/new.go": {1, 2}})

    def test_deleted_file_contributes_no_added_lines(self) -> None:
        added = c.parse_added_lines(DIFF_DELETED_FILE)
        self.assertEqual(added, {})

    def test_multiple_hunks_in_one_file_accumulate(self) -> None:
        diff = (
            "--- a/x.go\n"
            "+++ b/x.go\n"
            "@@ -1,0 +2,1 @@\n"
            "+first\n"
            "@@ -10,0 +20,1 @@\n"
            "+second\n"
        )
        self.assertEqual(c.parse_added_lines(diff), {"x.go": {2, 20}})

    def test_empty_diff_yields_no_files(self) -> None:
        self.assertEqual(c.parse_added_lines(""), {})


class ParseCoverageProfileTest(unittest.TestCase):
    def test_parses_blocks_and_skips_mode_header(self) -> None:
        blocks = c.parse_coverage_profile(PROFILE_TEXT)
        key = "github.com/wesleykirkland/terraform-provider-urllo/internal/provider/rule_types.go"
        self.assertIn(key, blocks)
        self.assertEqual(len(blocks[key]), 4)
        self.assertIn((55, 61, 0), blocks[key])
        self.assertIn((64, 69, 1), blocks[key])

    def test_blank_text_yields_no_blocks(self) -> None:
        self.assertEqual(c.parse_coverage_profile(""), {})


class FindUncoveredTest(unittest.TestCase):
    def test_flags_lines_inside_a_zero_hit_block(self) -> None:
        added = {"internal/provider/rule_types.go": {56, 57, 58}}
        profile = c.parse_coverage_profile(PROFILE_TEXT)
        uncovered = c.find_uncovered(added, profile)
        self.assertEqual(uncovered, {"internal/provider/rule_types.go": [56, 57, 58]})

    def test_does_not_flag_lines_inside_a_covered_block(self) -> None:
        added = {"internal/provider/rule_types.go": {64, 65}}
        profile = c.parse_coverage_profile(PROFILE_TEXT)
        self.assertEqual(c.find_uncovered(added, profile), {})

    def test_lines_outside_any_block_are_skipped_not_flagged(self) -> None:
        # e.g. a blank line or a closing brace, which go test never
        # instruments as its own statement.
        added = {"internal/provider/rule_types.go": {200}}
        profile = c.parse_coverage_profile(PROFILE_TEXT)
        self.assertEqual(c.find_uncovered(added, profile), {})

    def test_file_absent_from_profile_is_skipped_not_flagged(self) -> None:
        added = {"tools/tools.go": {1, 2, 3}}
        profile = c.parse_coverage_profile(PROFILE_TEXT)
        self.assertEqual(c.find_uncovered(added, profile), {})

    def test_matches_profile_key_by_path_suffix(self) -> None:
        # profile keys are module-qualified (github.com/.../x.go); added-line
        # keys are repo-relative (x.go) -- must match on suffix.
        added = {"internal/provider/rule_types.go": {56}}
        profile = c.parse_coverage_profile(PROFILE_TEXT)
        self.assertTrue(c.find_uncovered(added, profile))

    def test_marked_block_is_exempted_via_sources(self) -> None:
        added = {"internal/provider/rule_types.go": {56, 57, 58}}
        profile = c.parse_coverage_profile(PROFILE_TEXT)
        source_lines = ["" for _ in range(54)] + ["// coverage:ignore: reason"] + ["" for _ in range(10)]
        sources = {"internal/provider/rule_types.go": source_lines}
        self.assertEqual(c.find_uncovered(added, profile, sources), {})


class BlockIsIgnoredTest(unittest.TestCase):
    def test_true_when_marker_present_in_range(self) -> None:
        lines = ["if x {", "    // coverage:ignore: reason", "    y()", "}"]
        self.assertTrue(c.block_is_ignored(lines, 2, 3))

    def test_false_when_marker_absent(self) -> None:
        lines = ["if x {", "    y()", "}"]
        self.assertFalse(c.block_is_ignored(lines, 1, 3))

    def test_false_when_marker_outside_range(self) -> None:
        lines = ["// coverage:ignore: unrelated", "if x {", "    y()", "}"]
        self.assertFalse(c.block_is_ignored(lines, 2, 4))


class BlocksForFileTest(unittest.TestCase):
    def test_finds_blocks_by_suffix_match(self) -> None:
        profile = c.parse_coverage_profile(PROFILE_TEXT)
        blocks = c.blocks_for_file(profile, "internal/provider/rule_types.go")
        self.assertIsNotNone(blocks)
        self.assertEqual(len(blocks), 4)

    def test_returns_none_when_file_not_in_profile(self) -> None:
        profile = c.parse_coverage_profile(PROFILE_TEXT)
        self.assertIsNone(c.blocks_for_file(profile, "tools/tools.go"))


class LineIsUncoveredTest(unittest.TestCase):
    BLOCKS = [(55, 61, 0), (64, 69, 1)]

    def test_true_for_line_in_zero_hit_block_without_source(self) -> None:
        self.assertTrue(c.line_is_uncovered(56, self.BLOCKS, None))

    def test_false_for_line_in_covered_block(self) -> None:
        self.assertFalse(c.line_is_uncovered(65, self.BLOCKS, None))

    def test_false_for_line_outside_any_block(self) -> None:
        self.assertFalse(c.line_is_uncovered(200, self.BLOCKS, None))

    def test_false_when_block_carries_ignore_marker(self) -> None:
        source = ["" for _ in range(60)] + ["// coverage:ignore: reason"] + ["" for _ in range(10)]
        self.assertFalse(c.line_is_uncovered(56, self.BLOCKS, source))

    def test_true_when_source_present_but_unmarked(self) -> None:
        source = ["plain line" for _ in range(70)]
        self.assertTrue(c.line_is_uncovered(56, self.BLOCKS, source))


class LoadSourcesTest(unittest.TestCase):
    def test_reads_existing_files_and_skips_missing(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "a.go").write_text("line1\nline2\n", encoding="utf-8")
            sources = c.load_sources(root, ["a.go", "missing.go"])
        self.assertEqual(sources, {"a.go": ["line1", "line2"]})


class RepoRootTest(unittest.TestCase):
    def test_invokes_git_rev_parse(self) -> None:
        result = mock.Mock(stdout="/some/repo\n")
        with mock.patch.object(c.subprocess, "run", return_value=result) as run:
            root = c.repo_root()
        self.assertEqual(root, Path("/some/repo"))
        (args,), kwargs = run.call_args
        self.assertEqual(args, ["git", "rev-parse", "--show-toplevel"])
        self.assertTrue(kwargs["check"])


class MergeBaseTest(unittest.TestCase):
    def test_returns_stripped_sha_on_success(self) -> None:
        result = mock.Mock(returncode=0, stdout="abc123\n")
        with mock.patch.object(c.subprocess, "run", return_value=result):
            self.assertEqual(c.merge_base(Path("/repo")), "abc123")

    def test_returns_none_when_branch_unresolvable(self) -> None:
        result = mock.Mock(returncode=1, stdout="")
        with mock.patch.object(c.subprocess, "run", return_value=result):
            self.assertIsNone(c.merge_base(Path("/repo")))


class DiffAgainstTest(unittest.TestCase):
    def test_invokes_expected_git_diff_command(self) -> None:
        result = mock.Mock(stdout="diff text")
        with mock.patch.object(c.subprocess, "run", return_value=result) as run:
            out = c.diff_against(Path("/repo"), "base-sha")
        self.assertEqual(out, "diff text")
        (args,), kwargs = run.call_args
        self.assertEqual(
            args,
            ["git", "diff", "--unified=0", "base-sha", "--cached", "--", "*.go", ":(exclude)*_test.go"],
        )
        self.assertEqual(kwargs["cwd"], Path("/repo"))
        self.assertTrue(kwargs["check"])


class MainTest(unittest.TestCase):
    def test_returns_one_when_profile_missing(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            with mock.patch.object(c, "repo_root", return_value=root):
                self.assertEqual(c.main(), 1)

    def test_returns_zero_and_skips_when_base_unresolvable(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / c.COVERAGE_PROFILE).write_text("mode: set\n", encoding="utf-8")
            with (
                mock.patch.object(c, "repo_root", return_value=root),
                mock.patch.object(c, "merge_base", return_value=None),
            ):
                self.assertEqual(c.main(), 0)

    def test_returns_zero_when_all_added_lines_covered(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / c.COVERAGE_PROFILE).write_text(PROFILE_TEXT, encoding="utf-8")
            diff = "--- a/internal/provider/rule_types.go\n+++ b/internal/provider/rule_types.go\n@@ -63,0 +64,1 @@\n+covered line\n"
            with (
                mock.patch.object(c, "repo_root", return_value=root),
                mock.patch.object(c, "merge_base", return_value="base-sha"),
                mock.patch.object(c, "diff_against", return_value=diff),
            ):
                self.assertEqual(c.main(), 0)

    def test_returns_one_when_added_lines_uncovered(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / c.COVERAGE_PROFILE).write_text(PROFILE_TEXT, encoding="utf-8")
            diff = DIFF_ONE_FILE
            with (
                mock.patch.object(c, "repo_root", return_value=root),
                mock.patch.object(c, "merge_base", return_value="base-sha"),
                mock.patch.object(c, "diff_against", return_value=diff),
            ):
                self.assertEqual(c.main(), 1)

    def test_returns_zero_when_uncovered_block_is_marked_ignored_on_disk(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / c.COVERAGE_PROFILE).write_text(PROFILE_TEXT, encoding="utf-8")
            provider_dir = root / "internal" / "provider"
            provider_dir.mkdir(parents=True)
            # DIFF_ONE_FILE's added lines (52-58) span two uncovered profile
            # blocks, (52,54,0) and (55,61,0), so both need a marker line.
            src_lines = ["" for _ in range(70)]
            src_lines[52] = "// coverage:ignore: reason"  # line 53, in (52,54,0)
            src_lines[55] = "// coverage:ignore: reason"  # line 56, in (55,61,0)
            (provider_dir / "rule_types.go").write_text("\n".join(src_lines), encoding="utf-8")
            with (
                mock.patch.object(c, "repo_root", return_value=root),
                mock.patch.object(c, "merge_base", return_value="base-sha"),
                mock.patch.object(c, "diff_against", return_value=DIFF_ONE_FILE),
            ):
                self.assertEqual(c.main(), 0)

    def test_returns_zero_when_no_go_files_changed(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / c.COVERAGE_PROFILE).write_text(PROFILE_TEXT, encoding="utf-8")
            with (
                mock.patch.object(c, "repo_root", return_value=root),
                mock.patch.object(c, "merge_base", return_value="base-sha"),
                mock.patch.object(c, "diff_against", return_value=""),
            ):
                self.assertEqual(c.main(), 0)


if __name__ == "__main__":
    unittest.main()
