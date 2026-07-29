---
name: docgen
description: Regenerate this provider's docs/ (via tfplugindocs) and reconcile README.md / terraform/README.md against the current resources, data sources, and provider config options. Use after adding, renaming, or removing a resource, data source, or schema attribute -- or whenever `make check-docs` fails.
---

# docgen

Regenerates generated documentation and fixes drift in the two hand-written
READMEs so they stay accurate as the provider's schema changes.

## Steps

1. Run `make generate` from the repo root. Per `tools/tools.go`'s
   `go:generate` directives, this runs, in order:
   - `copywrite headers` -- adds/checks license headers
   - `terraform fmt -recursive examples/`
   - `tfplugindocs generate` -- rewrites everything under `docs/` from the
     live provider schema in `internal/provider/*.go`

   `docs/` is fully generated output. Never hand-edit a file under `docs/` --
   the next `make generate` overwrites it. If a `docs/` file is missing
   content, the fix is a schema/description change in the corresponding
   `internal/provider/*.go` file, not the doc.

2. Run `make check-docs` (`scripts/check_docs.py`). It fails with specific,
   actionable lines if:
   - a `internal/provider/<name>_resource.go` or `<name>_data_source.go`
     file (matched by that naming convention and its
     `resp.TypeName = req.ProviderTypeName + "_xxx"` line) has no
     corresponding `docs/resources/xxx.md` / `docs/data-sources/xxx.md`, or
     isn't linked from root `README.md`'s "Resources and Data Sources" table
   - a top-level attribute in `UrlloProvider.Schema()`
     (`internal/provider/provider.go`) isn't mentioned in root `README.md`'s
     provider configuration table
   - a `.tf` file under `terraform/` isn't mentioned in
     `terraform/README.md`'s "What the example does" list
   - a markdown formatting problem: an unbalanced code fence, a table row
     with a different column count than its header, a relative link that
     doesn't resolve, or (for a `docs/` file) a missing tfplugindocs
     generated-file header

3. For each failure, hand-edit the README the script named -- add the
   missing table row / bullet / link, matching the formatting already used
   there (existing table column alignment, the `docs/<kind>/<name>.md` link
   pattern, backtick-quoted attribute/file names). Re-run `make check-docs`
   until it exits 0.

4. Run `git diff --stat` and report what changed: which `docs/` files were
   regenerated versus which README lines you added by hand. Leave the
   changes unstaged and uncommitted.

Do not invent wording for something you haven't verified exists -- if
`check-docs` names an attribute or file, read the real Go source in
`internal/provider/` (or the actual file under `terraform/`) before writing
its description, rather than guessing from the name alone.

**Never run `git add`, `git commit`, or `git push` as part of this skill.**
Regenerate and edit files, then stop and report what changed. The user
reviews the diff and decides what to stage and commit -- that authority is
never this skill's to take, even when every check passes.
