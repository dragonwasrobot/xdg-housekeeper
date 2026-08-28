---
name: testing
description: How tests are written in this repo — test format and Given/When/Then structure, when a table-driven test is legitimate, assertion shape with go-cmp, integration coverage against temporary directories, what to test and what to leave to a dependency's own tests. Load before writing, changing, or reviewing any test.
---

# Testing conventions

**Test format.** One test function per behavior, named
`TestSubject_ShouldStateTheBehavior`, with BDD-style Given/When/Then comments
inside:

```go
// see internal/domain/xdg/xdg_test.go
func TestResolve_ShouldFallBackToTheSpecifiedDefaultsWhenNothingIsSet(t *testing.T) {
    // Given a home directory and no XDG variables are set
    environment := Environment{Home: "/home/dwr"}

    // When resolving the base directories
    got, err := Resolve(environment)
    if err != nil {
        t.Fatalf("resolve base directories: %v", err)
    }

    // Then every location falls back to its default under home
    want := BaseDirs{
        config: "/home/dwr/.config",
        data:   "/home/dwr/.local/share",
        state:  "/home/dwr/.local/state",
        cache:  "/home/dwr/.cache",
    }

    if diff := cmp.Diff(want, got, cmp.AllowUnexported(BaseDirs{})); diff != "" {
        t.Errorf("base directories mismatch (-want +got):\n%s", diff)
    }
}
```

**Table-driven tests are the exception, not the default.** The plain form above
is what a new test looks like; a table is what one occasionally becomes. Reach
for it only when all three hold: the unit is a pure input→output function; every
row exercises **one** behavior already pinned by its own Red → Green cycle; and
the rows differ in *data* only — not in shape, setup, or which assertion runs.
Resolving the same path against each XDG location fits. Anything where a row
needs its own `if` in the loop body, or carries a `wantErr` alongside rows that
want a value, does not — that is two behaviors, and it wants two functions.

```go
// see internal/domain/xdg/xdg_test.go
func TestTarget_ShouldJoinAPathOntoItsLocationsBaseDirectory(t *testing.T) {
    // Given base directories resolved from a home directory
    baseDirs, err := Resolve(Environment{Home: "/home/dwr"})
    if err != nil {
        t.Fatalf("resolve base directories: %v", err)
    }

    tests := []struct {
        name     string
        location Location
        relPath  string
        want     Target
    }{
        {
            name:     "should resolve a target under the config base",
            location: LocationConfig,
            relPath:  "git/config",
            want:     "/home/dwr/.config/git/config",
        },
        {
            name:     "should resolve a target under the data base",
            location: LocationData,
            relPath:  "gnupg",
            want:     "/home/dwr/.local/share/gnupg",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // When resolving a path relative to that location's base
            got, err := baseDirs.Target(tt.relPath, tt.location)
            if err != nil {
                t.Fatalf("resolve target: %v", err)
            }

            // Then the target exists inside the base directory
            if diff := cmp.Diff(tt.want, got); diff != "" {
                t.Errorf("target mismatch (-want +got):\n%s", diff)
            }
        })
    }
}
```

A table compresses behavior *already* driven out test-first; it never
substitutes for the cycle. A row for genuinely new behavior gets its own
Red → Green first and is folded in once green — which is also how a plain test
becomes a table: the second row arrives red, not as a refactor.

Examples inside doc comments document behavior as plain input/output pairs and
take no Given/When/Then comments, whether or not they are wired up as an
executable `func Example…`.

**Spec language.** Test names and Given/When/Then comments describe behavior in
domain terms, not implementation. Don't reference parameter names, function
calls, or sentinel names in the description itself. Setup and assertions use the
implementation freely; the names and comments are the spec layer.

- ✅ `TestTarget_ShouldRejectAPathThatDoesNotLiveInsideItsBaseDirectory` —
  behavior, in domain terms
- ❌ `TestTarget_ShouldReturnErrInvalidTargetWhenPathHasDotDot` — leaks the
  sentinel's name and the parsing mechanics

The name list is also the coverage report: `go test -v -run 'TestResolve_'`
prints one subject's behaviors in source order, and a missing input class reads
as a gap in that prose. Declare them in an order that makes it read that way.

**Assertion shape.** Assert on whole values with `github.com/google/go-cmp/cmp`
and report the diff:

```go
if diff := cmp.Diff(want, got); diff != "" {
    t.Errorf("plan mismatch (-want +got):\n%s", diff)
}
```

`cmp.Diff` takes `want` first — reversing it inverts the `-`/`+` legend and
misleads whoever reads the failure. Never `reflect.DeepEqual` on a struct: it
answers true/false where `cmp` shows which field moved — `forbidigo` bans it.
Diff the whole value rather than indexing into it: `got[0]` asserts on one
element and silently stops covering the rest. For a struct with unexported
fields, pass `cmp.AllowUnexported(T{})`; to exclude a volatile field such as a
timestamp, `cmpopts.IgnoreFields` — both beat asserting field by field, which
silently stops covering fields added later. For the error arm,
`errors.Is`/`errors.As` per the Guiding principles in `CLAUDE.md` — never
`err.Error()` text, which an upstream wrapping layer changes without any
behavior changing.

**Integration coverage.** Cover the orchestrating layer against real temporary
directories rather than a mocked filesystem. Use `t.TempDir()`: unlike a
trailing cleanup call, its removal still runs when an earlier assertion fails —
`usetesting` rejects `os.MkdirTemp` in its favour. Use `t.Cleanup` for anything
else needing an undo, and `t.Helper()` in shared assertion helpers, which
`thelper` enforces. For a pure function that only reads a filesystem,
`fstest.MapFS` beats a temp directory — the fixture states itself inline.

**What to test.**

- **Test the unit's own contribution, not its dependencies.** A function that
  delegates to a well-tested helper needs at most one test for what *it* adds. A
  one-line transformation usually needs none of its own — callers verify it
  transitively. Before adding an edge case, check whether the edge belongs to
  this unit or to something it calls.
- **Test an invariant where it is established, not where it surfaces.** A
  constraint enforced at construction — `Resolve` cleaning a base directory, or
  rejecting the filesystem root — is pinned by a test of the constructor,
  however far downstream the breakage would show. The tell: the behavior the
  name claims was already true of the value before the subject ran.
- **Changing a contract means editing the test that owns it, not appending a new
  one.** When behavior changes, find the existing test pinning the old contract
  and update it in place. Refining how an already-covered input is handled —
  normalizing, trimming, coercing — is *changed* behavior, however new it feels.
  The tell is duplicated setup: if the new test would re-stage the same fixture,
  call the same function, and assert on the same field as an existing one, it
  belongs *in* that existing test.
- **Orchestrator tests count.** Composition logic — sequencing, error
  propagation in both directions, derived values — is where regressions land.
  Pure helpers are easy to test and tempting to over-test while orchestrators
  are harder and quietly skipped; resist that asymmetry.
