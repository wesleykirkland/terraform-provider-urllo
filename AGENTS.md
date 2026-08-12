# Working in this repo

## Test coverage is not optional

This repo enforces coverage at two levels. Both are checked locally
(`.githooks/pre-commit`, installed automatically by any `make` target) and in
CI (`test.yml`'s `unit` job) -- a gap doesn't get to wait for a SonarCloud
report to surface.

1. **Repo-wide floor: 97%** (`make cover`, `COVER_MIN` in `GNUmakefile`). See
   [COVERAGE.md](COVERAGE.md) for the full breakdown and the small, explicitly
   documented set of unreachable defensive guards that keep this at 97%
   instead of 100%.

2. **New-code floor: 100%, no exceptions** (`make cover-new`,
   `NEW_CODE_COVER_MIN` in `GNUmakefile`). Every line you add or change on a
   branch, relative to `main`, must be exercised by a test. This is stricter
   than the repo-wide floor on purpose: a small new file can add a completely
   untested function without moving the aggregate 97% number at all -- that's
   exactly what happened with `internal/provider/rule_types.go`'s
   `analyticsToObject`, which shipped at 30% new-code coverage and only got
   caught by SonarCloud's PR gate, after the fact. `make cover-new`
   (`scripts/check_new_code_coverage.py`) approximates that same "coverage on
   new code" check locally, so the gap is caught before you commit rather
   than after a Sonar report comes back.

**Practical rule: write the test in the same commit as the code.** Don't
lean on the 97% aggregate floor to hide a gap in something you just added --
`make cover-new` will catch it anyway, and it's cheaper to add the test while
the code is fresh in your head than to come back to it later.

If a gap is a genuinely unreachable defensive guard (the same category
documented in COVERAGE.md's "Why not 100%" section), don't just leave it --
either restructure so it's actually reachable/testable, or document it in
COVERAGE.md the way the existing exceptions are, so a floor-lowering doesn't
silently mask a real regression next time.

Both `make cover` and `make cover-new` run the mock-backed acceptance tests
(`TestAccMock*`), which need no credentials -- only the *real* (non-mock)
acceptance tests (`TestAccRuleResource`, etc.) require Urllo credentials, and
those self-skip without them. So both coverage gates run the same in CI and
locally.
