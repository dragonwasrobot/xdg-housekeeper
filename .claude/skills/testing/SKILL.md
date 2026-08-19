---
name: testing
description: How tests are written in this repo — test format and Given/When/Then structure, when a table-driven test is legitimate, assertion shape with go-cmp, integration coverage against temporary directories, what to test and what to leave to a dependency's own tests. Load before writing, changing, or reviewing any test.
---

# Testing conventions

**Test format.** One test function per behavior, named
`TestSubject_ShouldStateTheBehavior`, with BDD-style Given/When/Then comments
inside:

```go
func TestResolve_ShouldRejectATargetOutsideTheHomeDirectory(t *testing.T) {
    // Given an entry whose target escapes the home directory
    entry := Entry{Name: "app", Path: "../../etc/app"}

    // When resolving it against that home directory
    _, err := Resolve(entry, "/home/dwr")

    // Then it reports an escaping-target failure
    if !errors.Is(err, ErrEscapingTarget) {
        t.Errorf("got error %v, want ErrEscapingTarget", err)
    }
}
```

**Table-driven tests are the exception, not the default.** Reach for a table
only when all three hold: the unit is a pure input→output function; every row
exercises **one** behavior already pinned by its own Red → Green cycle; and the
rows differ in *data* only — not in shape, setup, or which assertion runs.
Classifying a range of path spellings fits. Anything where a row needs its own
`if` in the loop body, or carries a `wantErr` alongside rows that want a value,
does not — that is two behaviors, and it wants two functions.

```go
func TestClassify_ShouldClassifyEachXDGLocation(t *testing.T) {
    tests := []struct {
        name string
        path string
        want Location
    }{
        {name: "should classify a config path", path: ".config/app", want: LocationConfig},
        {name: "should classify a data path", path: ".local/share/app", want: LocationData},
        {name: "should classify an unmanaged path", path: ".appconfig", want: LocationUnmanaged},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Given a home-relative path, when classifying it
            got := Classify(tt.path)

            // Then it maps to the expected XDG location
            if diff := cmp.Diff(tt.want, got); diff != "" {
                t.Errorf("location mismatch (-want +got):\n%s", diff)
            }
        })
    }
}
```

A table compresses behavior *already* driven out test-first; it never
substitutes for the cycle. A row for genuinely new behavior gets its own
Red → Green first and is folded in once green.

Examples inside doc comments document behavior as plain input/output pairs and
take no Given/When/Then comments, whether or not they are wired up as an
executable `func Example…`.

**Spec language.** Test names and Given/When/Then comments describe behavior in
domain terms, not implementation. Don't reference parameter names, function
calls, or sentinel names in the description itself. Setup and assertions use the
implementation freely; the names and comments are the spec layer.

- ✅ `TestResolve_ShouldRejectATargetOutsideTheHomeDirectory` — behavior, in
  domain terms
- ❌ `TestResolve_ShouldReturnErrEscapingTargetWhenPathHasDotDot` — leaks the
  sentinel's name and the parsing mechanics

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
