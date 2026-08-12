## Context Engine (CCE)

This project uses Code Context Engine for intelligent code retrieval and
cross-session memory.

### Searching the codebase

**Use `context_search` instead of reading files directly** when exploring
the codebase, answering questions about code, or understanding how things
work. `context_search` returns the most relevant code chunks with
confidence scores instead of whole files.

When to use `context_search`:
- Answering questions about the codebase ("how does X work?", "where is Y?")
- Exploring structure or architecture
- Finding related code, functions, or patterns

Other tools:
- `expand_chunk` for full source of a compressed result
- `related_context` for what calls/imports a function
- `session_recall` to recall past decisions

### Cross-session memory

Call `session_recall("topic phrase")` before answering non-trivial questions.
Call `record_decision(decision="...", reason="...")` after making choices.
Call `record_code_area(file_path="...", description="...")` after meaningful work.

### Output style

Respond in compressed style. Drop articles (a, an, the) in prose. Use
sentence fragments over full sentences. Use short synonyms (fix not resolve,
check not investigate). Pattern: [thing] [action] [reason]. [next step].
No filler, hedging, pleasantries, trailing summaries, or restating what
the user said. One sentence if one sentence is enough.

When suggesting code changes, show only the changed lines with 3 lines of
context. Never rewrite entire files. Multiple changes in one file: show each
change separately. Never echo back unchanged code the user already has.

Code blocks, file paths, commands, error messages: always written in full.
Security warnings and destructive action confirmations: use full clarity.

## Testing changes

Always run this before considering a change to `internal/` or `terraform/*.tf`
done — `go build`/`go test` passing is not sufficient proof it works end to
end:

```shell
./terraform/build-local.sh
terraform -chdir=terraform plan
```

`build-local.sh` builds the provider from source, installs it into the local
filesystem mirror, and re-inits `terraform/` against that build. `plan`
requires `URLLO_API_KEY` / `URLLO_API_SECRET` in the environment, since it
reads real hosts and rules from the live Urllo API — see `terraform/README.md`
for full setup. Review the plan output for unexpected diffs (e.g. attributes
that shouldn't have changed showing as changed) before reporting the task
complete.
