#!/usr/bin/env python3
# Copyright (c) Wesley Kirkland-Daily
# SPDX-License-Identifier: MPL-2.0

"""Warns when the live Urllo OpenAPI spec has moved past the version this
provider was built against.

API_SPEC_VERSION at the repo root records `info.version` from
https://dashboard.urllo.com/docs/api/openapi.json as of the last time
someone reviewed the spec and updated the provider for it. Urllo doesn't
publish a changelog for API changes, so diffing that field against the
live spec is the only drift signal available.

Allows soft failures

Run directly with:
    python3 scripts/check_api_spec_version.py
"""

from __future__ import annotations

import json
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path

SPEC_URL = "https://dashboard.urllo.com/docs/api/openapi.json"
VERSION_FILE = "API_SPEC_VERSION"
TIMEOUT_SECONDS = 10


def repo_root() -> Path:
    out = subprocess.run(
        ["git", "rev-parse", "--show-toplevel"],
        capture_output=True,
        text=True,
        check=True,
    )
    return Path(out.stdout.strip())


def built_version(root: Path) -> str:
    return (root / VERSION_FILE).read_text(encoding="utf-8").strip()


def fetch_live_version() -> str:
    with urllib.request.urlopen(SPEC_URL, timeout=TIMEOUT_SECONDS) as response:
        spec = json.load(response)
    return spec["info"]["version"]


def warn(message: str) -> None:
    # GitHub Actions annotation syntax -- surfaces in the PR checks UI and
    # the job summary without needing the step (or job) to exit non-zero.
    print(f"::warning::{message}")


def main() -> int:
    root = repo_root()
    built = built_version(root)

    try:
        live = fetch_live_version()
    except (urllib.error.URLError, TimeoutError, ValueError, KeyError) as exc:
        warn(f"Could not check the live Urllo OpenAPI spec ({SPEC_URL}) for drift: {exc}")
        return 0

    if live == built:
        print(f"Urllo OpenAPI spec version unchanged ({built}).")
        return 0

    warn(
        f"Urllo OpenAPI spec has moved from {built} (recorded in {VERSION_FILE}) to {live}. "
        f"Review {SPEC_URL} for changes relevant to this provider, then update {VERSION_FILE}."
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())  # pragma: no cover
