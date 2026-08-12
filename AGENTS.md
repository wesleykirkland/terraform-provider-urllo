# Working in this repo

## Test coverage is not optional

This repo enforces coverage at two levels, both reading their threshold from
the same `COVER_MIN` variable (`GNUmakefile`, default **97%**). Both are
checked locally (`.githooks/pre-commit`, installed automatically by any
`make` target) and in CI (`test.yml`'s `unit` job) -- a gap doesn't get to
wait for a SonarCloud report to surface.

1. **Repo-wide floor** (`make cover`): total statement coverage across the
   whole Go codebase. See [COVERAGE.md](COVERAGE.md) for the full breakdown
   and the small, explicitly documented set of unreachable defensive guards
   that keep this below 100%.

2. **New-code floor** (`make cover-new`): every line *added on this branch,
   relative to `main`*, must be covered -- unless explicitly marked
   unreachable (see below). This exists because a small new file can add a
   completely untested function without moving the repo-wide aggregate
   number at all -- that's exactly what happened with
   `internal/provider/rule_types.go`'s `analyticsToObject`, which shipped at
   30% new-code coverage and only got caught by SonarCloud's PR gate, after
   the fact. `make cover-new` (`scripts/check_new_code_coverage.py`)
   approximates that same "coverage on new code" check locally, so the gap
   is caught before you commit rather than after a Sonar report comes back.

Since both read `COVER_MIN`, override it per-invocation for a stricter local
check on new code specifically, e.g. `make cover-new COVER_MIN=100` --
setting it for one target's invocation doesn't affect the other's, since
each `make` command is a separate process.

**Practical rule: write the test in the same commit as the code.** Don't
lean on the aggregate floor to hide a gap in something you just added --
`make cover-new` will catch it (down to `COVER_MIN`) anyway, and it's cheaper
to add the test while the code is fresh in your head than to come back to it
later.

If a gap is a genuinely unreachable defensive guard (the same category
documented in COVERAGE.md's "Why not 100%" section) rather than something
that's merely untested, don't silently leave it uncovered and don't lower the
floor to paper over it either: restructure so it's actually reachable if you
can, or mark the block with a trailing `// coverage:ignore: <reason>`
comment and add it to COVERAGE.md's list, the way `host_resource.go`'s
`Read` nil-guard does. `make cover-new` treats that marker as the sanctioned
exemption -- an unmarked gap always fails the check.

Both `make cover` and `make cover-new` run the mock-backed acceptance tests
(`TestAccMock*`), which need no credentials -- only the *real* (non-mock)
acceptance tests (`TestAccRuleResource`, etc.) require Urllo credentials, and
those self-skip without them. So both coverage gates run the same in CI and
locally.
