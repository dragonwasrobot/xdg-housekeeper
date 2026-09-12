---
name: testing
description: How specs are written in this repo with Ginkgo and Gomega — Describe/Context/It format and Given/When/Then structure, when a DescribeTable is legitimate, assertion shape with BeComparableTo, integration coverage against temporary directories, what to test and what to leave to a dependency's own tests. Load before writing, changing, or reviewing any spec.
---

# Testing conventions

**Spec format.** One `It` per behavior, inside a `Describe` naming the subject
and a `Context` naming the condition, with BDD-style Given/When/Then comments
inside the spec body:

```go
// see internal/domain/xdg/xdg_test.go
var _ = Describe("Resolve", func() {
    Context("when no XDG variables are set", func() {
        It("should fall back to the specified defaults", func() {
            // Given a home directory and no XDG variables are set
            environment := Environment{Home: "/home/dwr"}

            // When resolving the base directories
            baseDirs, err := Resolve(environment)

            // Then every location falls back to its default under home
            Expect(err).NotTo(HaveOccurred())
            Expect(baseDirs).To(BeComparableTo(BaseDirs{
                config: "/home/dwr/.config",
                data:   "/home/dwr/.local/share",
                state:  "/home/dwr/.local/state",
                cache:  "/home/dwr/.cache",
            }, cmp.AllowUnexported(BaseDirs{})))
        })
    })
})
```

The three descriptions concatenate into one sentence in the output — `Resolve
when no XDG variables are set should fall back to the specified defaults` — so
write them to read that way. The behavior string is lowercase, prefixed
`should`, and followed by an infinitive: `should fall back`, never `should falls
back`. Drop the `Context` where the condition adds nothing, and let two
conditions share a behavior string when they really do assert the same thing.

Every spec file dot-imports the same two packages:

```go
import (
    . "github.com/onsi/ginkgo/v2/dsl/core"
    . "github.com/onsi/gomega"
)
```

A file holding a table adds `"github.com/onsi/ginkgo/v2/dsl/table"` beside
them; a file without one must not, since an unused import does not compile.

The split DSL packages, never the top-level `ginkgo/v2` — its `Entry` collides
with the `plan.Entry` domain type, and `package plan` will not compile.
`dsl/core` and Gomega are dot-imported, which ST1001 permits only because
`.golangci.yml` whitelists exactly those two paths; `dsl/table` keeps its own
name, so `table.DescribeTable` and `table.Entry` read the same everywhere and
the colliding package gets no special case. A dot import is per-file, so each
spec file repeats the block.

Each package has one bootstrap file — `xdg_suite_test.go`, `plan_suite_test.go`
— holding the only `func Test…` the package still has:

```go
func TestXdg(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "xdg")
}
```

`Fail` is Ginkgo's, `RegisterFailHandler` is Gomega's. The bootstrap goes in the
package under test rather than an external `_test` package, because specs
construct `BaseDirs{config: …}` and call unexported `rules()`.

**Table-driven tests are the exception, not the default.** The plain form above
is what a new test looks like; a table is what one occasionally becomes. Reach
for it only when all three hold: the unit is a pure input→output function; every
row exercises **one** behavior already pinned by its own Red → Green cycle; and
the rows differ in *data* only — not in shape, setup, or which assertion runs.
Resolving the same path against each XDG location fits. Anything where a row
needs its own `if` in the shared body, or carries a `wantErr` alongside rows
that want a value, does not — that is two behaviors, and it wants two specs.

```go
// see internal/domain/xdg/xdg_test.go
var _ = Describe("Target", func() {
    var baseDirs BaseDirs

    BeforeEach(func() {
        // Given base directories resolved from a home directory
        var err error

        baseDirs, err = Resolve(Environment{Home: "/home/dwr"})
        Expect(err).NotTo(HaveOccurred())
    })

    table.DescribeTable("should join a path onto its location's base directory",
        func(location Location, relPath string, want Target) {
            // When resolving a path relative to that location's base
            target, err := baseDirs.Target(relPath, location)

            // Then the target exists inside the base directory
            Expect(err).NotTo(HaveOccurred())
            Expect(target).To(BeComparableTo(want))
        },
        table.Entry("for the config location",
            LocationConfig, "git/config",
            Target("/home/dwr/.config/git/config")),
        table.Entry("for the data location",
            LocationData, "gnupg",
            Target("/home/dwr/.local/share/gnupg")),
    )
})
```

Each `Entry`'s arguments are passed positionally to the shared body, so a row
can only be data — the third condition above, now enforced by a signature
rather than by discipline. Wrap a row's arguments across lines as above; the
`lll` exclusion exists for the description, not as licence for a 120-column
row.

An `Entry` description is the discriminator and is appended after the table's,
so the two are written to join up. Where rows differ by a value, the row names
that value — `for the config location`. Where they differ by *why* the same
behavior holds, the condition goes in a `Context` and each row starts `because`:

```
Target when the path does not point inside its base directory
  should refuse to name a target at all because it is absolute
```

A table compresses behavior *already* driven out test-first; it never
substitutes for the cycle. A row for genuinely new behavior gets its own
Red → Green first and is folded in once green — which is also how a plain spec
becomes a table: the second row arrives red, not as a refactor.

Examples inside doc comments document behavior as plain input/output pairs and
take no Given/When/Then comments, whether or not they are wired up as an
executable `func Example…`.

**Spec language.** Spec descriptions and Given/When/Then comments describe
behavior in domain terms, not implementation. Don't reference parameter names,
function calls, or sentinel names in the description itself. Setup and
assertions use the implementation freely; the descriptions and comments are the
spec layer.

- ✅ `Context("when the path does not point inside its base directory")` /
  `It("should refuse to name a target at all")` — behavior, in domain terms
- ❌ `It("should return ErrInvalidTarget when the path has ..")` — leaks the
  sentinel's name and the parsing mechanics

The spec tree is also the coverage report:

```sh
go test ./internal/domain/xdg/ -v -ginkgo.v
```

prints every subject's behaviors in source order, and adding
`-ginkgo.focus='Resolve'` narrows it to one. Both flags are needed: `-ginkgo.v`
alone is swallowed by `go test`'s output buffering, and `-v` alone prints dots
rather than names. A missing input class reads as a gap in that prose, so
declare containers in an order that makes it read that way.

**Fixtures.** A value more than one spec spells out *identically* gets a fixture
at the foot of the spec file, named for what it is — `gitRule()`, never
`makeRule()` or `newFixture()`. Repetition of a shape is not repetition of a
value: a `Survey` literal opens nearly every spec in `plan_test.go` and stays
inline in each, because what fills it is what that spec is about.

A fixture with structure is a function taking no arguments and returning the
value whole. One that is a bare scalar is a `const` — `bashObstacle` — since
there is nothing for a body to assemble. A spec needing a variant binds the
fixture and changes the one field that differs, so the deviation is what the
reader sees:

```go
// see internal/domain/plan/plan_test.go
bogus := gitRule()
bogus.Relocation = Relocation("bogus")
```

What makes one spec differ from its neighbours stays inline. Entry paths,
targets, and the occupied map are written out in the spec body, because a
constructor hiding those leaves the reader crossing the file to learn what is
being asserted. The bar for extracting is repetition across specs, never length
within one.

A fixture belongs in an expectation as readily as in the setup — the `Claim`
spec asserts `Rule: gitRule()` — as long as it is **pinned**: some spec already
using it fails if the fixture drifts from what it copies. `gitRule()` restates a
row of `rules()` and `Claim` consults that real table, so a changed row breaks
the spec; `bashObstacle` is pinned the same way through `unsupportedApps()`. The
`Rules` specs do not do that job — they assert invariants over whatever the
table holds, never the values in it.

A fixture that copies nothing needs no pin. `lessRule()` and `wgetRule()` name
rules `rules()` never declares, and every subject reaching them — `Build` by
argument, `Targets` through its receiver — is handed its candidates by the spec
rather than consulting the table. They are values a spec invents, with nothing
to drift from.

A fixture that copies a row and has no pin is the case to fix, not to document.
Never add a spec comparing the fixture against the row: that pins the copy and
covers no behavior. Either route a spec that uses it through the real table, or
stop copying — invent a value and name it for what the spec needs.

**Assertion shape.** Assert on whole values with `BeComparableTo`, which runs
`github.com/google/go-cmp/cmp` underneath and reports its diff:

```go
Expect(candidates).To(BeComparableTo(want))
```

**Never `Equal`.** That matcher is `reflect.DeepEqual` and answers true/false
where `BeComparableTo` shows which field moved. `forbidigo` bans
`reflect.DeepEqual` outright, but Gomega calls it from inside the matcher where
the linter cannot see it, so the rule lives here instead of in `.golangci.yml`.
It is absolute — it holds for a scalar too. Reach for the matcher that names
the property instead: `HaveLen` for a length, `ContainElement` for membership.

`BeComparableTo` passes `actual` to `cmp.Diff` first, so its legend is
`-actual +expected` — the reverse of `cmp.Diff(want, got)`, and `+` is the side
the spec asked for. Diff the whole value rather than indexing into it —
`candidates[0]` asserts on one element and
silently stops covering the rest. `cmp` options pass straight through, so
`cmp.AllowUnexported(T{})` covers a struct with unexported fields and
`cmpopts.IgnoreFields` excludes a volatile one such as a timestamp; both beat
asserting field by field, which silently stops covering fields added later.

For the error arm, `MatchError(ErrInvalidTarget)` — it tries `errors.Is` first,
so the sentinel-identity rule from `CLAUDE.md` §Error handling carries over
intact. Never assert on `err.Error()` text, which an upstream wrapping layer
changes without any behavior changing. Where a value is only meaningful if the
call succeeded, guard it with `Expect(err).NotTo(HaveOccurred())`: a failed
`Expect` aborts the spec, so it does the job the old `t.Fatalf` guard did and
the assertion below it never runs against a zero value.

**Ginkgo mechanics.** The container tree is built at *load time*: the closures
passed to `Describe` and `Context` run once, before any spec, purely to register
what is inside them. Only `It`, `BeforeEach`, and their siblings run at spec
time. A statement written directly in a `Describe` body therefore executes once
and leaks its value into every spec below it. All setup goes in `BeforeEach`,
against a `var` declared at container level — the `Target` table above is the
shape. Where every container in a file wants the same fixture, a top-level
`var _ = BeforeEach(…)` against a package-level `var` applies to the whole
suite, which is how `plan_test.go` resolves its base directories once.

That `var` looks like shared mutable state and is not: `BeforeEach` re-runs
before every spec, so each one gets a fresh value. Two ways to get it wrong,
both of which compile and fail silently:

- **Assigning in an `It`.** The value is built for that spec alone, and every
  other spec reads whatever the previous one left behind.
- **`:=` inside `BeforeEach`.** That declares a *new* `baseDirs` shadowing the
  container-level one, which then keeps its zero value forever. Declare
  `var err error` and assign both with `=`.

Ginkgo randomises the order of top-level containers on every run, which turns
accidental coupling between specs into a failure rather than a latent bug. Each
run prints its seed; pass it back as `-ginkgo.seed` to reproduce an ordering.

**Integration coverage.** Cover the orchestrating layer against real temporary
directories rather than a mocked filesystem. There is no `*testing.T` inside a
spec, so use `GinkgoT().TempDir()`: like `t.TempDir()`, its removal still runs
when an earlier assertion fails, where a trailing cleanup call would be skipped.
Use `DeferCleanup` for anything else needing an undo — it registers against the
node that calls it, so a `BeforeEach` cleanup runs after each spec — and
`GinkgoHelper()` in a shared assertion helper, so a failure reports the caller's
line instead of the helper's. For a pure function that only reads a filesystem,
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
