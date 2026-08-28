---
name: refactor
description: The labeled-change cycle for structured refactors in this repo — restructuring, renaming, or fixing principle violations across multiple files, where the single-test-per-cycle TDD loop doesn't fit. Load before starting a refactor, and when resuming one already in progress. Covers the numbered cycle, the verification step, and what the loop forbids.
---

# Refactor workflow

Refactoring spans multiple files or principles and doesn't fit the
single-test-per-cycle TDD loop. The discipline is different but the goal is the
same: keep cycles tight and keep the developer in the loop.

The cycle for a structured refactor:

1. Enumerate the changes up front — a numbered list of violations, files, or
   principles in scope. Surface it before touching any code.
2. Apply **one** labeled change — its own tool call with a short label (e.g.
   "change 3/4", "finding #2") so it's reviewable in isolation.
3. Review that change against the **Guiding principles** in `CLAUDE.md` — the
   same judgment check as the feature loop's Blue step, watching especially for
   violations the refactor itself introduced: a new long parameter list, broken
   declaration order, a premature abstraction, an error that lost its wrapping
   context on the way through.
4. **Verification** — run this before handing anything back, so the developer
   never reads a diff that hasn't been checked:

   ```sh
   mise run verify
   ```

   It runs `go vet`, `golangci-lint` (gofmt included), and `go test`, stopping
   at the first failure.

   A Stop hook runs the same command when the tree has Go changes, so a
   blocked stop means the diff is not ready. It checks a chain only once, so
   it does not replace this step.
5. **Stop and hand the diff back.** End the turn so the developer reads the
   change before the next lands — that human read is the second review. A
   labeled change may span more than one edit; finish it, then yield.

**Forbidden:**

- Beginning the next labeled change before the developer has reviewed the
  previous one — finish a change, then yield the turn. Sequential edits in one
  response are still a batch, even as separate tool calls.
- Batching unrelated edits into a single tool call.
- Applying a fix that touches multiple principles in a single change.
- Continuing past a `mise run verify` failure without surfacing it.
- Renaming a domain term in code without carrying it into `CONTEXT.md` and the
  tests in the same change.

**When automated verification isn't possible** (flag wiring, real-home
behavior): replace the verify step with a handoff — describe the golden path and
any relevant edge cases for the developer to exercise manually.
