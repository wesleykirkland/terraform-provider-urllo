#!/usr/bin/env python3
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
"""

from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path

FAILURES: list[str] = []


def fail(msg: str) -> None:
    FAILURES.append(msg)


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


def check_resources_and_data_sources(root: Path, readme: str) -> None:
    provider_dir = root / "internal" / "provider"
    paths = sorted(provider_dir.glob("*_resource.go")) + sorted(provider_dir.glob("*_data_source.go"))
    for path in paths:
        if path.name.endswith("_test.go"):
            continue

        is_data_source = path.stem.endswith("_data_source")
        kind = "data-sources" if is_data_source else "resources"

        text = path.read_text(encoding="utf-8")
        match = TYPE_NAME_RE.search(text)
        if not match:
            fail(
                f'{path.relative_to(root)} has no \'resp.TypeName = req.ProviderTypeName + "_xxx"\' '
                f"line -- can't determine its type name"
            )
            continue

        suffix = match.group(1)
        name = f"urllo_{suffix}"
        docfile = root / "docs" / kind / f"{suffix}.md"

        if not docfile.is_file():
            fail(
                f"{docfile.relative_to(root)} (schema for {name} in {path.relative_to(root)} "
                f"has no generated doc; run 'make generate')"
            )

        if f"docs/{kind}/{suffix}.md" not in readme:
            fail(f"README.md does not link {docfile.relative_to(root)} (for {name})")


# --- Provider configuration options --------------------------------------
#
# Every top-level attribute in UrlloProvider's Schema() should be mentioned
# (as a backtick-quoted argument name) in README.md's configuration table.
ATTR_RE = re.compile(r'^\s*"([a-z_]+)":\s*schema\.', re.MULTILINE)


def check_provider_config(root: Path, readme: str) -> None:
    provider_go = root / "internal" / "provider" / "provider.go"
    text = provider_go.read_text(encoding="utf-8")
    for attr in ATTR_RE.findall(text):
        if f"`{attr}`" not in readme:
            fail(f"README.md's provider configuration table does not mention `{attr}`")


# --- Terraform example files ----------------------------------------------
#
# Every terraform/*.tf file should be mentioned in terraform/README.md's
# "What the example does" list.
def check_terraform_examples(root: Path) -> None:
    tf_readme = (root / "terraform" / "README.md").read_text(encoding="utf-8")
    for tf_file in sorted((root / "terraform").glob("*.tf")):
        if f"`{tf_file.name}`" not in tf_readme:
            fail(f"terraform/README.md does not mention `{tf_file.name}`")


# --- Markdown formatting --------------------------------------------------
#
# Structural sanity checks on the markdown itself, independent of content:
# unbalanced code fences, ragged tables, and relative links that don't
# resolve. Applied to the hand-written READMEs plus every generated doc
# (which should also catch a docs/ file that got hand-edited into a broken
# state instead of regenerated).
FENCE_RE = re.compile(r"^```")
LINK_RE = re.compile(r"\]\(([^)]+)\)")


def check_markdown_format(root: Path) -> None:
    files = [root / "README.md", root / "terraform" / "README.md"]
    docs_dir = root / "docs"
    if docs_dir.is_dir():
        files += sorted(docs_dir.rglob("*.md"))

    for path in files:
        if not path.is_file():
            continue
        rel = path.relative_to(root)
        text = path.read_text(encoding="utf-8")
        lines = text.splitlines()

        fence_count = sum(1 for line in lines if FENCE_RE.match(line))
        if fence_count % 2 != 0:
            fail(f"{rel} has an unbalanced number of ``` code fences ({fence_count})")

        in_fence = False
        in_table = False
        header_pipes = 0
        for lineno, line in enumerate(lines, start=1):
            if FENCE_RE.match(line):
                in_fence = not in_fence
                continue
            if in_fence:
                continue
            if line.startswith("|"):
                pipes = line.count("|")
                if not in_table:
                    in_table = True
                    header_pipes = pipes
                elif pipes != header_pipes:
                    fail(f"{rel}:{lineno}: table row has {pipes} pipes, header has {header_pipes}")
                continue
            in_table = False

        directory = path.parent
        for link in LINK_RE.findall(text):
            if re.match(r"^[a-z]+://", link):
                continue
            if link.startswith("#"):
                continue
            link_path = link.split("#", 1)[0]
            if not link_path:
                continue
            if not (directory / link_path).exists():
                fail(f"{rel} links to '{link}', which does not resolve to {directory / link_path}")


# Generated docs should still carry the tfplugindocs header; if it's
# missing, the file was probably hand-edited instead of produced by
# `make generate`.
def check_generated_headers(root: Path) -> None:
    docs_dir = root / "docs"
    if not docs_dir.is_dir():
        return
    for path in sorted(docs_dir.rglob("*.md")):
        head = "\n".join(path.read_text(encoding="utf-8").splitlines()[:5])
        if "generated by https://github.com/hashicorp/terraform-plugin-docs" not in head:
            fail(
                f"{path.relative_to(root)} is missing the tfplugindocs generated-file header -- "
                f"was it hand-edited? Run 'make generate'."
            )


def main() -> int:
    root = repo_root()
    readme = (root / "README.md").read_text(encoding="utf-8")

    check_resources_and_data_sources(root, readme)
    check_provider_config(root, readme)
    check_terraform_examples(root)
    check_markdown_format(root)
    check_generated_headers(root)

    if FAILURES:
        for msg in FAILURES:
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
    sys.exit(main())
