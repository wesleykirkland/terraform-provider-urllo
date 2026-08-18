#!/usr/bin/env python3
# Copyright Wesley Kirkland-Daily 2026
# SPDX-License-Identifier: MPL-2.0

"""Checks that README.md / terraform/README.md reference every resource,
data source, provider config option, and example .tf file that actually
exists in the code, and that the relevant markdown files are structurally
well-formed.

This does NOT re-verify that docs/ itself is fresh relative to the schema
-- that's already covered by `make generate` + the CI "generate" job's
`git diff --exit-code`. This script only catches drift in the two
hand-maintained READMEs (plus basic markdown hygiene), which nothing else
checks. Pure standard library so it runs the same on Linux, macOS, and
Windows without a POSIX shell.

Each check_* function takes the repo root (and, where relevant, README text
already read by the caller) and returns a list of failure message strings --
no shared mutable state -- so scripts/test_check_docs.py can call them
directly against a synthetic directory tree.
"""

from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path

README_NAME = "README.md"


def repo_root() -> Path:
    out = subprocess.run(
        ["git", "rev-parse", "--show-toplevel"],
        capture_output=True,
        text=True,
        check=True,
    )
    return Path(out.stdout.strip())


# --- Resources & data sources ------------------------------------------
#
# Every internal/provider/<name>_resource.go or <name>_data_source.go sets
# `resp.TypeName = req.ProviderTypeName + "_<suffix>"` in its Metadata func.
# For each one, confirm the corresponding generated doc exists and that
# README.md links to it.
TYPE_NAME_RE = re.compile(r'resp\.TypeName\s*=\s*req\.ProviderTypeName\s*\+\s*"_([a-z_]+)"')


def check_resources_and_data_sources(root: Path, readme: str) -> list[str]:
    failures: list[str] = []
    provider_dir = root / "internal" / "provider"
    paths = sorted(provider_dir.glob("*_resource.go")) + sorted(provider_dir.glob("*_data_source.go"))
    for path in paths:
        is_data_source = path.stem.endswith("_data_source")
        kind = "data-sources" if is_data_source else "resources"

        text = path.read_text(encoding="utf-8")
        match = TYPE_NAME_RE.search(text)
        if not match:
            failures.append(
                f'{path.relative_to(root)} has no \'resp.TypeName = req.ProviderTypeName + "_xxx"\' '
                f"line -- can't determine its type name"
            )
            continue

        suffix = match.group(1)
        name = f"urllo_{suffix}"
        docfile = root / "docs" / kind / f"{suffix}.md"

        if not docfile.is_file():
            failures.append(
                f"{docfile.relative_to(root)} (schema for {name} in {path.relative_to(root)} "
                f"has no generated doc; run 'make generate')"
            )

        if f"docs/{kind}/{suffix}.md" not in readme:
            failures.append(f"README.md does not link {docfile.relative_to(root)} (for {name})")

    return failures


# --- Provider configuration options --------------------------------------
#
# Every top-level attribute in UrlloProvider's Schema() should be mentioned
# (as a backtick-quoted argument name) in README.md's configuration table.
ATTR_RE = re.compile(r'^\s*"([a-z_]+)":\s*schema\.', re.MULTILINE)


def check_provider_config(root: Path, readme: str) -> list[str]:
    failures: list[str] = []
    provider_go = root / "internal" / "provider" / "provider.go"
    text = provider_go.read_text(encoding="utf-8")
    for attr in ATTR_RE.findall(text):
        if f"`{attr}`" not in readme:
            failures.append(f"README.md's provider configuration table does not mention `{attr}`")
    return failures


# --- Terraform example files ----------------------------------------------
#
# Every terraform/*.tf file should be mentioned in terraform/README.md's
# "What the example does" list.
def check_terraform_examples(root: Path) -> list[str]:
    failures: list[str] = []
    tf_readme = (root / "terraform" / README_NAME).read_text(encoding="utf-8")
    for tf_file in sorted((root / "terraform").glob("*.tf")):
        if f"`{tf_file.name}`" not in tf_readme:
            failures.append(f"terraform/README.md does not mention `{tf_file.name}`")
    return failures


# --- Markdown formatting --------------------------------------------------
#
# Structural sanity checks on the markdown itself, independent of content:
# unbalanced code fences, ragged tables, and relative links that don't
# resolve. Applied to the hand-written READMEs plus every generated doc
# (which should also catch a docs/ file that got hand-edited into a broken
# state instead of regenerated).
FENCE_RE = re.compile(r"^```")
LINK_RE = re.compile(r"\]\(([^)]+)\)")


def _check_fence_balance(rel: Path, lines: list[str]) -> list[str]:
    fence_count = sum(1 for line in lines if FENCE_RE.match(line))
    if fence_count % 2 != 0:
        return [f"{rel} has an unbalanced number of ``` code fences ({fence_count})"]
    return []


def _check_table_columns(rel: Path, lines: list[str]) -> list[str]:
    failures: list[str] = []
    in_fence = False
    in_table = False
    header_pipes = 0
    for lineno, line in enumerate(lines, start=1):
        if FENCE_RE.match(line):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        if not line.startswith("|"):
            in_table = False
            continue
        pipes = line.count("|")
        if not in_table:
            in_table = True
            header_pipes = pipes
        elif pipes != header_pipes:
            failures.append(f"{rel}:{lineno}: table row has {pipes} pipes, header has {header_pipes}")
    return failures


def _check_relative_links(rel: Path, text: str, directory: Path) -> list[str]:
    failures: list[str] = []
    for link in LINK_RE.findall(text):
        if re.match(r"^[a-z]+://", link):
            continue
        if link.startswith("#"):
            continue
        # LINK_RE requires at least one character inside the parens, and a
        # link starting with "#" is already skipped above, so splitting off
        # the anchor here can never leave an empty link_path.
        link_path = link.split("#", 1)[0]
        if not (directory / link_path).exists():
            failures.append(f"{rel} links to '{link}', which does not resolve to {directory / link_path}")
    return failures


def check_markdown_format(root: Path) -> list[str]:
    failures: list[str] = []
    files = [root / README_NAME, root / "terraform" / README_NAME]
    docs_dir = root / "docs"
    if docs_dir.is_dir():
        files += sorted(docs_dir.rglob("*.md"))

    for path in files:
        if not path.is_file():
            continue
        rel = path.relative_to(root)
        text = path.read_text(encoding="utf-8")
        lines = text.splitlines()

        failures += _check_fence_balance(rel, lines)
        failures += _check_table_columns(rel, lines)
        failures += _check_relative_links(rel, text, path.parent)

    return failures


# Generated docs should still carry the tfplugindocs header; if it's
# missing, the file was probably hand-edited instead of produced by
# `make generate`.
def check_generated_headers(root: Path) -> list[str]:
    failures: list[str] = []
    docs_dir = root / "docs"
    if not docs_dir.is_dir():
        return failures
    for path in sorted(docs_dir.rglob("*.md")):
        head = "\n".join(path.read_text(encoding="utf-8").splitlines()[:5])
        if "generated by https://github.com/hashicorp/terraform-plugin-docs" not in head:
            failures.append(
                f"{path.relative_to(root)} is missing the tfplugindocs generated-file header -- "
                f"was it hand-edited? Run 'make generate'."
            )
    return failures


def run_all_checks(root: Path) -> list[str]:
    readme = (root / README_NAME).read_text(encoding="utf-8")
    failures: list[str] = []
    failures += check_resources_and_data_sources(root, readme)
    failures += check_provider_config(root, readme)
    failures += check_terraform_examples(root)
    failures += check_markdown_format(root)
    failures += check_generated_headers(root)
    return failures


def main() -> int:
    failures = run_all_checks(repo_root())

    if failures:
        for msg in failures:
            print(f"MISSING: {msg}")
        print()
        print(
            "Documentation is out of sync with the code. Update the README(s) above",
            file=sys.stderr,
        )
        print(
            "(the 'docgen' Claude skill can do this), then re-run 'make check-docs'.",
            file=sys.stderr,
        )
        return 1

    print("check-docs: OK")
    return 0


if __name__ == "__main__":
    sys.exit(main())  # pragma: no cover
