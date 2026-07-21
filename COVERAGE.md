# Test coverage

This provider is tested to a high statement-coverage level and enforces a floor
in CI (`make cover`, currently **97%**):

| Package             | Coverage |
| ------------------- | -------- |
| `main`              | 100%     |
| `internal/client`   | 100%     |
| `internal/provider` | ~98%     |

Coverage comes from three layers:

1. **Unit tests** — the API client (`httptest` for auth, envelope decoding,
   pagination, idempotency keys, 429/Retry-After backoff, error mapping, and DNS
   validation with an injected resolver) and the provider's pure conversion
   helpers.
2. **White-box tests** — provider `Configure` with unknown values, and each
   resource/data-source method driven with a deliberately-unreadable request to
   exercise the "cannot decode request" guards.
3. **Mock-backed acceptance tests** (`TestAccMock*`) — the full CRUD path run
   through the real Terraform plugin protocol against an in-memory Urllo API,
   including create/read/update/delete errors, disappeared resources, delete
   idempotency, and DNS-validation timeouts. These need no credentials.

## Why not 100%

A handful of statements are **unreachable defensive guards** that cannot execute
at runtime with a valid schema, and are intentionally kept:

- `if resp.Diagnostics.HasError() { return }` immediately after a successful
  `Plan.Get` / `State.Get` / helper call. The framework cannot fail to decode a
  value that already conforms to the schema, so the guard never fires — but it is
  the pattern HashiCorp's own scaffolding uses, and removing it would drop a real
  safety net if the schema and model ever diverge.
- `if host == nil` in the host resource's `Read`, a nil-guard for the
  name-lookup path that `Read` (which always has an ID) never takes.

These are deliberately **not** removed to reach a round number; correctness and
robustness are preferred over a 100% figure. The CI floor (`COVER_MIN`) guards
against regressions in the coverable code.
