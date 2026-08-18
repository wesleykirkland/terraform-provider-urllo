#!/usr/bin/env python3
# Copyright Wesley Kirkland-Daily 2026
# SPDX-License-Identifier: MPL-2.0

"""Unit tests for check_api_spec_version.py.

fetch_live_version() and repo_root() are mocked throughout -- these tests
check the comparison/warning logic, not that urllib or git actually work
(and they must not make real network calls during `make test-py`).
"""

from __future__ import annotations

import sys
import tempfile
import unittest
from contextlib import contextmanager
from pathlib import Path
from typing import Iterator
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parent))

import check_api_spec_version  # noqa: E402


@contextmanager
def temp_repo(version: str = "1.2.0") -> Iterator[Path]:
    with tempfile.TemporaryDirectory() as tmp:
        root = Path(tmp)
        (root / check_api_spec_version.VERSION_FILE).write_text(f"{version}\n", encoding="utf-8")
        yield root


class BuiltVersionTest(unittest.TestCase):
    def test_reads_and_strips_version_file(self) -> None:
        with temp_repo("1.2.0") as root:
            self.assertEqual(check_api_spec_version.built_version(root), "1.2.0")

    def test_strips_surrounding_whitespace(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / check_api_spec_version.VERSION_FILE).write_text("\n  1.3.0  \n", encoding="utf-8")
            self.assertEqual(check_api_spec_version.built_version(root), "1.3.0")


class RepoRootTest(unittest.TestCase):
    def test_invokes_git_rev_parse(self) -> None:
        result = mock.Mock(stdout="/some/repo\n")
        with mock.patch.object(check_api_spec_version.subprocess, "run", return_value=result) as run:
            root = check_api_spec_version.repo_root()
        self.assertEqual(root, Path("/some/repo"))
        (args,), kwargs = run.call_args
        self.assertEqual(args, ["git", "rev-parse", "--show-toplevel"])
        self.assertTrue(kwargs["check"])


class FetchLiveVersionTest(unittest.TestCase):
    def test_parses_info_version_from_response_body(self) -> None:
        response = mock.MagicMock()
        response.__enter__.return_value = response
        response.read.return_value = b'{"info": {"version": "1.2.0"}}'
        with mock.patch.object(check_api_spec_version.urllib.request, "urlopen", return_value=response):
            self.assertEqual(check_api_spec_version.fetch_live_version(), "1.2.0")


class WarnTest(unittest.TestCase):
    def test_prints_github_actions_warning_annotation(self) -> None:
        with mock.patch("builtins.print") as mock_print:
            check_api_spec_version.warn("something drifted")
        mock_print.assert_called_once_with("::warning::something drifted")


class MainTest(unittest.TestCase):
    def test_returns_none_and_no_warning_when_versions_match(self) -> None:
        with temp_repo("1.2.0") as root:
            with (
                mock.patch.object(check_api_spec_version, "repo_root", return_value=root),
                mock.patch.object(check_api_spec_version, "fetch_live_version", return_value="1.2.0"),
                mock.patch.object(check_api_spec_version, "warn") as warn,
            ):
                self.assertIsNone(check_api_spec_version.main())
            warn.assert_not_called()

    def test_returns_none_and_warns_when_live_version_is_newer(self) -> None:
        with temp_repo("1.2.0") as root:
            with (
                mock.patch.object(check_api_spec_version, "repo_root", return_value=root),
                mock.patch.object(check_api_spec_version, "fetch_live_version", return_value="1.3.0"),
                mock.patch.object(check_api_spec_version, "warn") as warn,
            ):
                self.assertIsNone(check_api_spec_version.main())
            self.assertEqual(warn.call_count, 1)
            (message,), _ = warn.call_args
            self.assertIn("1.2.0", message)
            self.assertIn("1.3.0", message)

    def test_returns_none_and_warns_when_live_spec_is_unreachable(self) -> None:
        with temp_repo("1.2.0") as root:
            with (
                mock.patch.object(check_api_spec_version, "repo_root", return_value=root),
                mock.patch.object(
                    check_api_spec_version,
                    "fetch_live_version",
                    side_effect=check_api_spec_version.urllib.error.URLError("timed out"),
                ),
                mock.patch.object(check_api_spec_version, "warn") as warn,
            ):
                self.assertIsNone(check_api_spec_version.main())
            self.assertEqual(warn.call_count, 1)
            self.assertIn("Could not check", warn.call_args[0][0])

    def test_returns_none_and_warns_on_malformed_live_spec(self) -> None:
        with temp_repo("1.2.0") as root:
            with (
                mock.patch.object(check_api_spec_version, "repo_root", return_value=root),
                mock.patch.object(
                    check_api_spec_version,
                    "fetch_live_version",
                    side_effect=KeyError("version"),
                ),
                mock.patch.object(check_api_spec_version, "warn") as warn,
            ):
                self.assertIsNone(check_api_spec_version.main())
            self.assertEqual(warn.call_count, 1)


if __name__ == "__main__":
    unittest.main()
