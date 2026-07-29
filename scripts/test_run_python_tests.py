#!/usr/bin/env python3
# Copyright (c) Wesley Kirkland-Daily
# SPDX-License-Identifier: MPL-2.0

"""Unit tests for run_python_tests.py.

subprocess.run and shutil.which are mocked throughout -- these tests check
that run_python_tests.py builds the right commands and propagates the right
exit code, not that `coverage`/unittest actually work (that's what
test_check_docs.py and coverage.py's own test suite are for). Mocking also
avoids run_plain()/run_with_coverage() recursively re-invoking this same test
suite as a real subprocess.
"""

from __future__ import annotations

import sys
import unittest
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parent))

import run_python_tests  # noqa: E402


def fake_result(returncode: int) -> mock.Mock:
    result = mock.Mock()
    result.returncode = returncode
    return result


class RunPlainTest(unittest.TestCase):
    def test_invokes_unittest_discover_with_this_interpreter(self) -> None:
        with mock.patch.object(run_python_tests.subprocess, "run", return_value=fake_result(0)) as run:
            code = run_python_tests.run_plain()
        self.assertEqual(code, 0)
        (args,), kwargs = run.call_args
        self.assertEqual(args[0], sys.executable)
        self.assertIn("unittest", args)
        self.assertIn("discover", args)
        self.assertIn(run_python_tests.TEST_PATTERN, args)
        self.assertEqual(kwargs["cwd"], run_python_tests.ROOT)

    def test_propagates_nonzero_returncode(self) -> None:
        with mock.patch.object(run_python_tests.subprocess, "run", return_value=fake_result(1)):
            self.assertEqual(run_python_tests.run_plain(), 1)


class RunWithCoverageTest(unittest.TestCase):
    def test_runs_test_xml_and_report_steps_in_order(self) -> None:
        with mock.patch.object(run_python_tests.subprocess, "run", return_value=fake_result(0)) as run:
            code = run_python_tests.run_with_coverage("coverage")
        self.assertEqual(code, 0)
        self.assertEqual(run.call_count, 3)

        test_call, xml_call, report_call = run.call_args_list
        test_args = test_call.args[0]
        self.assertEqual(test_args[0], "coverage")
        self.assertEqual(test_args[1], "run")
        self.assertTrue(any(a.startswith("--source=") for a in test_args))
        self.assertTrue(any(a.startswith("--omit=") for a in test_args))

        xml_args = xml_call.args[0]
        self.assertEqual(xml_args[:2], ["coverage", "xml"])
        self.assertIn(str(run_python_tests.COVERAGE_XML), xml_args)

        report_args = report_call.args[0]
        self.assertEqual(report_args[:2], ["coverage", "report"])
        self.assertTrue(any(a.startswith("--fail-under=") for a in report_args))

    def test_omits_fail_under_when_coverage_min_is_zero(self) -> None:
        with mock.patch.object(run_python_tests, "COVERAGE_MIN", 0.0):
            with mock.patch.object(run_python_tests.subprocess, "run", return_value=fake_result(0)) as run:
                run_python_tests.run_with_coverage("coverage")
        report_args = run.call_args_list[-1].args[0]
        self.assertFalse(any(a.startswith("--fail-under=") for a in report_args))

    def test_test_failure_takes_precedence_over_later_steps(self) -> None:
        results = [fake_result(1), fake_result(0), fake_result(0)]
        with mock.patch.object(run_python_tests.subprocess, "run", side_effect=results) as run:
            code = run_python_tests.run_with_coverage("coverage")
        self.assertEqual(code, 1)
        # All three steps still run even though the test step failed, so
        # coverage.xml/the report are still produced for CI to inspect.
        self.assertEqual(run.call_count, 3)

    def test_xml_failure_takes_precedence_over_report(self) -> None:
        results = [fake_result(0), fake_result(2), fake_result(0)]
        with mock.patch.object(run_python_tests.subprocess, "run", side_effect=results):
            code = run_python_tests.run_with_coverage("coverage")
        self.assertEqual(code, 2)

    def test_report_failure_is_returned_when_others_pass(self) -> None:
        results = [fake_result(0), fake_result(0), fake_result(3)]
        with mock.patch.object(run_python_tests.subprocess, "run", side_effect=results):
            code = run_python_tests.run_with_coverage("coverage")
        self.assertEqual(code, 3)


class MainTest(unittest.TestCase):
    def test_falls_back_to_plain_when_coverage_missing(self) -> None:
        with mock.patch.object(run_python_tests.shutil, "which", return_value=None):
            with mock.patch.object(run_python_tests, "run_plain", return_value=0) as plain:
                with mock.patch.object(run_python_tests, "run_with_coverage") as with_cov:
                    code = run_python_tests.main()
        self.assertEqual(code, 0)
        plain.assert_called_once()
        with_cov.assert_not_called()

    def test_uses_coverage_when_available(self) -> None:
        with mock.patch.object(run_python_tests.shutil, "which", return_value="/usr/bin/coverage"):
            with mock.patch.object(run_python_tests, "run_with_coverage", return_value=0) as with_cov:
                with mock.patch.object(run_python_tests, "run_plain") as plain:
                    code = run_python_tests.main()
        self.assertEqual(code, 0)
        with_cov.assert_called_once_with("/usr/bin/coverage")
        plain.assert_not_called()


if __name__ == "__main__":
    unittest.main()
