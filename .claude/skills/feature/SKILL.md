---
name: feature
description: The test-first TDD cycle (Red → Green → Blue) for new functionality in this repo. Load before writing any code for a new feature, behavior, or capability — and when resuming a cycle already in progress. Covers the numbered cycle, the four verification commands, what the loop forbids, and the handoff to use when unit tests aren't possible.
---

# Feature workflow

New functionality is developed test-first, following a TDD cycle (**Red →
Green → Blue**). Never write production code without a failing test motivating
it. The developer stays in the loop on each iteration; keep cycles tight.

The cycle for each behavior:

1. Enumerate the behaviors to implement — a numbered list — and surface
   clarifying questions before writing any code.
2. If a new package or file is needed, create it with a typed stub: package doc
   comment, correct signatures and types, bodies that
   `panic("not implemented")`. No logic yet. **The stub must compile and be
   referenced from the test**, because Go fails the whole package on an unknown
   identifier — an unwired stub yields a build error, which is not a Red.
3. Write **exactly one** test for the next behavior (labeled, e.g. "behavior
   2/5") — load the `testing` skill and follow it — and confirm the test fails
   (**Red**). The failure must be an assertion failure or the stub's
   `not implemented` panic, never a build error.
4. Implement **only** what this specific test requires (**Green**). Do not write
   logic that no currently-failing test exercises — that belongs to a future
   cycle. A sign the Green was too broad: the next test you write passes
   immediately, with no code change.
5. Review the diff against the **Guiding principles** in `CLAUDE.md` (**Blue**)
   — a judgment review, not a tool run. Common targets: a missing doc comment on
   a new exported identifier, an error wrapped with `%v` instead of `%w` or
   returned without context, a bare `any`, a primitive standing in for a domain
   type, functions approaching the ~50 LOC ceiling, an unnamed multi-clause
   conditional, declaration order, whitespace. Refactor before moving on.
6. **Verification** — run all four before moving on:

   ```sh
   mise exec -- gofmt -l .
   mise exec -- go vet ./...
   mise exec -- golangci-lint run
   mise exec -- go test ./...
   ```

   `gofmt -l` printing any filename is a failure, even though it exits zero. A
   green suite confirms the mechanics, not the Blue review — it is no substitute
   for step 5.
7. **Stop and hand the diff back.** End the turn so the developer reads the
   cycle's diff before the next one starts. Never begin the next test in the
   same response.
8. Repeat from step 3 until all behaviors are covered.

**Forbidden:**

- Writing production code before a test fails for it.
- Writing more than one test before running the test suite.
- Implementing logic that no currently-failing test requires.
- Deleting or skipping existing tests to make a change land.
- Marking work as "tests later".
- Starting the next cycle's test before the developer has reviewed the current
  one's diff.

**When unit tests aren't possible** — flag wiring and exit codes in `main.go`,
or anything that must touch the real home directory rather than a `t.TempDir()`
— skip the Red → Green cycle, but replace it with a handoff: describe the golden
path and any relevant edge cases for the developer to exercise manually, and do
not mark the work done until they confirm. `go vet` and `golangci-lint run`
still run.
