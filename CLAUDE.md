# CLAUDE.md

## Development workflow

Work proceeds in tight cycles, with the developer reading the diff at the end
of each one. Only what binds *before* a skill loads is stated here.

- **New behavior → the `feature` skill.** Development is test-first (**Red →
  Green → Blue**): **never write production code without a failing test
  motivating it**, and never start the next cycle in the same response as the
  current one.
- **Restructuring existing code → the `refactor` skill.** **Apply one labeled
  change, then stop and hand the diff back** — sequential edits in one response
  are still a batch, even as separate tool calls.
- **Writing or changing a test → the `testing` skill.** Load it before writing
  the test, not to check one after.

## Architecture

Single binary, `package main` at the repo root. Everything else lives under
`internal/`, split into a pure core and the adapters around it:

```
main.go                 package main — flags, exit codes, wiring
internal/
  domain/               the pure core: no I/O, no env, no clock
    xdg/                package xdg    — XDG base-directory spec knowledge
    rules/              package rules  — the rule table, and unsupported apps
    plan/               package plan   — building a housekeeping plan
  infra/                the adapters: argv, filesystem, terminal text
    fsys/               package fsys   — traversing and moving files
    cli/                package cli    — argv parsing
    report/             package report — the plan and its outcomes, as text
```

An adapter translates between one outside format and the core's types, and
holds nothing else: `report` assembles text but writes no stream, and the
function that writes one stays in `main.go`. That splits the two `plan.Action`
switches: `actionLines` renders, `applyAction` relocates.

Directories carry the **layer**; package names carry the **concern**. So call
sites read `plan.Claim(entries, baseDirs)` and `fsys.Traverse(home, baseDirs)`,
never `domain.PlanCandidates(...)`. Add packages under a layer as concerns
appear; never add a package named for a layer, a pattern, or a catch-all
(`util`, `common`, `helpers`) — if a new package's one-concern sentence is
hard to write, it shouldn't exist yet.

Dependency direction is one-way: **an adapter may name a domain type; nothing
under `internal/domain/` may import the world.** The core is pure by its own
imports, not by who imports it — `depguard` in `.golangci.yml` denies it `os`,
`io`, `bufio`, `net`, `time` and `internal/infra` itself, so a violation fails
`golangci-lint run`.
`main` wires the two together: it calls an adapter, hands what comes back to
the core, and calls an adapter again with the result.

So an adapter returns the core's types rather than a mirror of them —
`fsys.Traverse` returns `[]plan.Entry`, `fsys.Probe` returns
`map[xdg.Target]plan.TargetState` — and `main` converts nothing in between.
Never add a package for the two layers to share.

Nothing under `internal/` carries a compatibility obligation to anyone outside
the repo, so reshape it freely.

**Probe in `scratchpad/`, not a temp directory outside the repo.** It is
git-ignored and holds throwaway probes — a file that deliberately trips a
linter, written to confirm what the config actually catches rather than what it
is assumed to. **Point the checker at the repo's own config, never a copy of
the rules**; a hand-written config confirms only what the copy catches.

A probe must stay invisible to `mise run verify`. For Go, give `scratchpad/`
its own `go.mod`; for a checker fed by `git ls-files`, git-ignoring is already
enough.

## Guiding principles

The principle that outranks the rest: optimize for the next reader. A clever
one-liner that costs five minutes of head-scratching loses to the boring
version. A rule naming a linter is caught by `mise run verify`; the rest, by
reading the diff.

### Reading the code

- **Let code breathe.** Blank lines frame multi-line `if`/`for`/`switch`
  blocks, so the reader sees where one thought ends and the next begins.
  `wsl_v5` enforces the exact shape and auto-fixes it.
- **Keep a line inside 80 columns**, so code sits beside the prose describing
  it — the same width `scripts/check-doc-width` holds Markdown to. `lll`
  enforces it.
- **Declare callers above callees.** If function A calls function B, declare A
  above B — same for unexported helpers. A type goes above the functions that
  build it. The reader sees intent before mechanism.
- **Prefer short functions.** `funlen` enforces a 40 LOC ceiling, but the real
  concern is single responsibility: treat approaching it as a signal the
  function is doing several things, and extract named helpers before adding
  more.
- **Express a collection transformation with a helper from `lo`.**
  `lo.FilterMap`, `lo.Filter`, `lo.Reject`, and `lo.KeyBy` say what a `for`
  leaves implicit in its `continue`s, and a named callback keeps the predicate
  legible. Write the loop instead where a step returns an error: a callback
  cannot, so the error would have to be collected outside the call — which is
  why `plan.Claim` is a loop.
- **`defer` cleanup immediately after acquiring.** The `defer f.Close()` goes on
  the line after the successful open, never at the end of the function — the
  pairing stays visible and no early return can skip it. That holds for a file
  you only read. For one you wrote to, `Close` reports flush failures — capture
  and return that error instead of deferring it away.
- **Open every package with a doc comment naming its concern.** A short
  paragraph directly above `package x`, phrased `// Package x …`; move it into
  a `doc.go` once it outgrows that or wants an example. `revive`'s
  `package-comments` rule enforces it.
- **Give every exported identifier a doc comment** — functions, types, methods,
  constants, and sentinel errors alike, internal plumbing included — starting
  with its own name (`// Claim surveys …`), per Go convention; `revive`'s
  `exported` rule enforces that much. Say *what* the thing does and *why* a
  caller would reach for it, never *how* — the body shows the how. A negation
  ("does X, not Y") earns its place only against an expectation the reader can
  see: a sibling function, a stdlib default, an adjacent type — never a design
  you discarded. Unexported functions get a comment only when they meet the
  same bar; trivial pass-throughs skip it.

### Types and signatures

- **Order parameters by relevance.** The value the function is fundamentally
  about leads; contextual criteria follow; injected config and clients trail:
  `plan.Claim(entries, baseDirs)`. When two parameters both read as the
  subject, ask which one could be frozen into the receiver and still leave a
  sensible operation: `b.In(location).Target(relPath)` reads, the reverse does
  not, so `location` is the criterion and trails. The one exception is
  `ctx context.Context`, which always comes first — that convention outranks
  this rule, and `revive`'s `context-as-argument` rule enforces it.
- **Keep the signature honest — it is the spec.** No bare `any`/`interface{}`
  standing in for a type you could name: a generic function takes a type
  parameter, and `...any` crosses a boundary only where a stdlib variadic such
  as `slog`'s dictates it. No primitive standing in for a domain concept — a
  resolved destination path is an `xdg.Target` and the base directory it
  belongs in is an `xdg.Location`, never a pair of `string`s; if a function
  takes three `bool`s, it wants an options struct.
- **No naked returns.** Name result parameters only when they document an
  otherwise-ambiguous same-typed pair, or when a deferred closure must set the
  error — capturing a write-side `Close` failure is the case that wants it.
  Return values explicitly even then. `nakedret` enforces it.
- **An `error`-only return is for an operation whose entire result is whether
  it happened.** `os.Rename`, `io.Closer` and `fsys.Move` are shaped that way.
  What such a return must not do is swallow a result the caller then has to
  recompute: when the operation determined a status, a count, or a path,
  return it.

### Boundaries

- **Functional core, imperative shell.** §Architecture gives the package split;
  this is the rule behind it: anything computational is a function of its
  arguments. `internal/infra` imports `os` and calls it directly — that is
  what an adapter is for, and it spares `main` constructing a handle to pass
  in. What infra must not do is hold the world between calls: no package-level
  handle, no `init`, no memoized singleton. Pass a dependency in as an
  argument only when it has a second real implementation or is
  non-deterministic. `slog` is the deliberate exception — a cross-cutting
  singleton, called directly rather than threaded through signatures.
- **No package-level mutable variables.** `var Err…` sentinels excepted;
  `gochecknoglobals` enforces it.
- **Accept interfaces, return structs.** Declare an interface at the *consumer*,
  not beside the implementation, and give it only the methods that consumer
  calls — a one-method interface is the norm. Prefer an existing stdlib
  interface to a new one: `writePlan(w io.Writer, …)` takes `os.Stdout` in
  `main` and a `bytes.Buffer` in its test. That second implementation is the
  bar — an interface with only one is a parameter pretending to be a seam.
  Return concrete types so callers keep every field and method without a type
  assertion.
- **Compose structs across package boundaries; embed them only within one.** A
  struct holds another package's struct as a named field rather than embedding
  it, so a change there can't ripple outward through an embedding chain.
  Embedding a struct is fine within a package. Interfaces are the exception at
  every boundary:
  embedding is how they compose, and a decorator embeds the interface it wraps
  so it can override one method and inherit the rest.
- **Validate untrusted input once, at the edge that reads it.** Flag parsing,
  environment reads, and file contents are where a value gets checked; nothing
  deeper repeats that check. A function still rejects what only it can judge —
  `BaseDirs.Target` returns `ErrInvalidTarget` for a path that escapes its base
  directory — because that is a fact about the result it computed, not a
  re-check of where its arguments came from.

### Naming

- **Keep one ubiquitous language across code and docs.** Every domain concept
  has exactly one name, used identically in type names, doc comments, tests,
  and prose. `CONTEXT.md` is the registry — check it before naming something
  new, and coining a genuinely new domain term means adding it there in the
  same change. `Housekeeper.Terms` fails a run on a registered retired
  spelling, but reads neither Markdown inline code nor a Ginkgo spec
  description, and a near-synonym nobody registered is invisible to it.
- **Name receivers with one or two letters.** `func (b BaseDirs) Target(…)`,
  never `func (baseDirs BaseDirs)` — the receiver is the subject of every line
  in the body and adds nothing the type has not already said. `staticcheck`
  catches only `this`/`self`/`me`, so length is on you.
- **Scale a name to its scope.** A loop index is `i`; a value used well below
  where it was bound earns a fuller name. Length tracks the distance a reader
  carries the name, not the importance of what it holds.
- **Name your conditionals.** Bind a complex boolean to a named local before
  branching on it, or extract a complex predicate into a named function.
  `if !insideBase` beats re-reading the conjunction at the branch — see
  `insideBase` and `within` in `internal/domain/xdg/xdg.go`.
- **Name your transformation chains.** When a contiguous run of statements
  exists only to turn one value into another, and the rest of the function does
  something else with the result, that run wants a name — a signal that fires
  at any length, where `funlen` only fires at 40. Bind the run to a well-named
  local when it just sequences already-named helpers; extract a named helper
  when it carries logic of its own: new predicates, new error cases, a result
  worth naming.
- **Alias deep paths.** From the second access onward, bind a multi-segment
  path's shared prefix to a local: `a.B.C.X` and `a.B.C.Y` collapse into one
  noun the surrounding code is about, and the reader stops re-traversing the
  path. A single access stays as it is.

### Restraint

- **Don't reach for concurrency without a requirement.** No goroutines,
  channels, or mutexes until there is a measured reason. A sequential CLI is
  easier to read, test, and debug — and here concurrent actions touching the
  same parent directory would be a correctness problem, not a speed win.
- **Don't abstract until the second use.** A few similar lines beat a premature
  abstraction: the first occurrence stays as it is, and the extraction waits
  for a second call site to actually land.

## Modelling data

- **Make impossible states unrepresentable.** Where a constraint can live in
  the type, put it there: `xdg.BaseDirs` keeps unexported fields behind
  `Resolve`, so a half-resolved value cannot be constructed at all. The
  alternative — an optional field, or a loose type admitting combinations that
  never occur — moves that constraint into every caller and every test.
- **Reserve a `*T` field for real absence.** Before modelling a field as `*T`,
  ask whether the absent state can actually occur — an optional field that can
  never be empty is a lie the reader has to disprove. When absence exists only
  at the source, resolve it at the boundary so it never enters a domain type.
  When absence is real *and* indistinguishable from the zero value, use `*T` or
  a `(T, bool)` pair — consistently, one convention per concept, never a mix.
- **Prefer value semantics; don't mutate what you didn't make.** Pass and
  return small structs by value; a pointer parameter or result is for a type
  that is large, needs identity, or must genuinely be mutated. Under
  `internal/domain`, never mutate a slice, map, or struct received as an
  argument — copy first if you need a changed version. The exception is
  `internal/infra` implementing a stdlib interface built to fill a
  caller-allocated buffer, as `io.Reader` is; mutation the caller can't see
  coming is a bug in either layer.

### Modelling finite variants

Two encodings, chosen by whether the variants carry data. Variants with no
payload get a const block:

```go
// internal/domain/xdg/xdg.go — abridged

// Location specifies which XDG base directory an entry belongs in.
type Location string

const (
    // LocationConfig specifies the base directory holding configuration.
    LocationConfig Location = "config"
    // LocationCache specifies the base directory holding non-essential,
    // regenerable data.
    LocationCache Location = "cache"
)
```

Variants carrying differing payloads get a sealed interface, closed by an
unexported method. Keep the fields of a variant the package **returns**
unexported and reach them through accessors, so nothing outside can fabricate
one and a fact every variant carries can live on the interface rather than in a
type switch. A variant the package **accepts** exports its fields instead: a
caller has to construct it, and Go cannot stop a zero-value composite literal,
so a constructor there is ceremony rather than a constraint. Name them as the
domain language already does — `EntryPath` is `Entry.Path`, `Target` is
`Candidate.Target` — never a fresh `From`/`To` pair for a path named elsewhere:

```go
// internal/domain/plan/plan.go — abridged

//sumtype:decl
type Action interface {
    isAction()

    // EntryPath returns the path in home of the entry this action covers.
    EntryPath() string
}

// Managed is an action whose entry an app owns, which is every action but
// Unmanaged.
//
//sumtype:decl
type Managed interface {
    Action

    // App returns the app that owns the entry.
    App() string
}

// Relocation is an action that moves its entry to a target, rather than
// leaving it where it is.
//
//sumtype:decl
type Relocation interface {
    Action

    // Target returns the path the entry relocates to.
    Target() xdg.Target
}

// Move relocates the entry to its target, leaving nothing behind.
type Move struct {
    app       string
    entryPath string
    target    xdg.Target
}

// App implements Managed.
func (m Move) App() string { return m.app }

// EntryPath implements Action.
func (m Move) EntryPath() string { return m.entryPath }

// Target implements Relocation.
func (m Move) Target() xdg.Target { return m.target }

// Unmanaged skips the entry, because no rule claims it. It is neither Managed
// nor a Relocation: nothing here knows which app the entry belongs to, and the
// entry stays where it is.
type Unmanaged struct {
    entryPath string
}

// EntryPath implements Action.
func (u Unmanaged) EntryPath() string { return u.entryPath }

func (Move) isAction()      {}
func (Unmanaged) isAction() {}
```

Two variants of several. One variant per case, never a single `Skip` carrying a
reason: the payloads differ, so one struct could only hold their union, and
`Unmanaged` — which has neither app nor target — would leave both fields empty.
That is the optional-field lie §Modelling data forbids.

A value **every** variant carries goes on the base interface, where a forgotten
variant is a compile error rather than a lint finding — here that is
`EntryPath` alone. A value a **subset** carries goes on a capability interface
embedding `Action`, and the consumer takes that instead:
`actionLines(action Managed)` never has to ask whether an app exists, and
`gochecksumtype` checks its switch against the facet's own variants. A value
**one** variant carries stays a plain accessor on it, as `Blocked.Blocker` is.

A facet declares no marker method of its own where the variants sit in one file
and a spec pins the behavior. `isAction` already seals everything embedding
`Action`, so a second marker adds no protection from outside — only from a
variant joining the facet by accident, at a method per variant. That leaves
membership structural, so two facets must not collide on a signature: the
target a relocation moves to is `Target`, and the one an `Occupied` skip
reports is `OccupiedTarget`.

Document the contract on the interface method, and write
`// X implements Managed.` on each variant — naming the interface that declares
the method, not always `Action`.

Branch on either with `switch`, never an `if`/`else` chain. Adding a variant
must break every switch that ignores it, and a `default` clause never excuses
a missing case.

The two forms want opposite endings. A typed constant is not closed at run
time — `Location("bogus")` is representable, and a bare `switch` matching
nothing silently does nothing — so give it a `default` that panics, naming the
value it rejected:

```go
// internal/domain/xdg/xdg.go — the tail of BaseDir

    default:
        panic(fmt.Sprintf("undefined location %q", location))
    }
}
```

The variants are compiled in, so a value outside them is programmer error, not
input to report — never widen a signature to `(T, error)` for a failure no
reachable input produces. A variant parsed from untrusted text is validated at
the edge that reads it instead, per §Boundaries, so the switch downstream still
panics.

A sealed interface needs no such `default`: its unexported method closes it, so
the clause could only ever catch a nil interface value. A type switch inside a
function that returns a value still needs an ending the compiler can see, so
the panic goes after the switch rather than inside it, naming the type it could
not place:

```go
// internal/infra/report/report.go — the tail of actionLines

    }

    panic(fmt.Sprintf("undefined action %T", action))
}
```

## Error handling

- A function with an expected failure mode — a missing directory, a malformed
  config line, a target outside its XDG base directory — returns `(T, error)`.
- Declare sentinels at package level: `var ErrInvalidTarget =
  errors.New("target is not inside an XDG base directory")`. Error strings are
  lowercase and unpunctuated, so they read correctly wrapped into a longer
  chain. `errname` enforces `ErrXxx` for sentinels and `XxxError` for error
  types; `staticcheck`'s ST1005 catches the trailing punctuation, the lowercase
  opening is on you.
- Wrap with context at every layer that has some to add:
  `fmt.Errorf("resolve %q: %w", name, err)`. A layer with nothing to add passes
  the error through untouched, rather than restating what the wrap below it
  already said.
- Which verb is an API decision, not a formatting one. `%w` publishes the
  wrapped error's identity as part of what this package promises, so reach for
  it when callers should branch on that identity — `plan.Claim` wraps
  `xdg.ErrInvalidTarget` with it, so a caller can still tell a target outside
  its base directory from any other failure. Use `%v` when identity is not the
  point: a raw `*os.PathError` or errno crossing out of `internal/infra` is an
  implementation detail, and `%w` there commits to it forever. A deliberate
  `%v` in a wrap carries `//nolint:errorlint // …` naming the detail being kept
  in; `nolintlint` enforces that the reason is there, not that it says
  anything.
- Callers branch with `errors.Is` for identity and `errors.As` when they need
  data out of the failure; a failure carrying data wants its own error type.
  Reach for `errors.AsType[E]` only where that data is used: discarding its
  first result trips `errcheck`'s `check-blank`, and a branch wanting the test
  alone stays on `errors.As`. Never compare `err.Error()` strings — `errorlint`
  catches `==` against a sentinel, the string comparison it won't.
- Never discard an error; `errcheck` and `ineffassign` enforce it.
- `panic` is for violated invariants and programmer error, never for bad input.
- `main` is the dispatcher: it writes the whole chain to stderr as one line and
  exits non-zero. It is the only place that chooses an exit code.

```go
// internal/domain/xdg/xdg.go — doc comments elided
var ErrInvalidTarget = errors.New("target is not inside an XDG base directory")

func (b BaseDirs) Target(relPath string, location Location) (Target, error) {
    basePath := b.BaseDir(location)
    targetPath := filepath.Join(basePath, relPath)
    insideBase := !filepath.IsAbs(relPath) && within(targetPath, basePath)

    if !insideBase {
        return "", fmt.Errorf("resolve %q: %w", relPath, ErrInvalidTarget)
    }

    return Target(targetPath), nil
}

// internal/domain/plan/plan.go — each layer wraps with the context it has
targetPath, err := baseDirs.Target(rule.RelPath, rule.Location)
if err != nil {
    return Survey{}, fmt.Errorf("resolve target for %q: %w", rule.App, err)
}

// main.go — the chain arrives whole, and only run picks the exit code
if err != nil {
    return reportFailure(stderr, errorLine(err), exitCodeFor(applied))
}

func errorLine(err error) string {
    return fmt.Sprintf("xdg-housekeeper: %v\n", err)
}
```

## Logging conventions

- **`log/slog` is for diagnostics; `fmt.Fprintln(os.Stdout, …)` is for
  CLI-user-facing text; `fmt.Fprintln(os.Stderr, …)` is for the dispatcher's
  error output.** That last split is the one that matters: the message printed
  when a run fails is this tool's answer to the person who ran it, not a
  diagnostic for an observability backend — so it never goes through `slog`.
  Never `log.Printf`, and never `fmt.Println` for a diagnostic — `forbidigo`
  bans the stdlib `log` package and the unqualified `fmt.Print*` calls outright.
- **Handle the error a stdout write returns.** `errcheck` excludes
  `fmt.Fprint*` to `os.Stderr` but nothing for `os.Stdout`, and buffering does
  not exempt it. Funnel user-facing output through one function that writes and
  returns a single error, rather than checking at every call site:

  ```go
  func writePlan(w io.Writer, actions []plan.Action) error {
      for _, action := range actions {
          if _, err := fmt.Fprintln(w, format(action)); err != nil {
              return fmt.Errorf("write plan: %w", err)
          }
      }

      return nil
  }
  ```

- **Four levels.** `debug` for argument and computed-value dumps; `info` for
  milestones; `warn` for an anomaly the run survives, such as an unreadable
  entry that gets skipped; `error` exactly once, at the dispatcher — its
  stderr line stands in for it until a verbosity flag configures a handler.
  `internal/domain` and `internal/infra` return errors — they do not log them.
  A failure logged where it happens *and* returned gets reported twice, and the
  duplicate is the one without the surrounding context.
- **Message shape.** `sloglint` enforces the structured key/value form over
  interpolation: the message is what you grep for, the attributes are what you
  filter on.

  ```go
  slog.Debug("parsed arguments", "args", args)
  slog.Info("moved entry", "from", src, "to", dst)
  slog.Warn("skipping unreadable entry", "path", path, "err", err)

  fmt.Fprintf(os.Stderr, "xdg-housekeeper: %v\n", err)
  ```
- **A dry run is output, not logging.** What the tool would do goes to stdout as
  user-facing text, at every verbosity — it is the answer to the question the
  user asked, so it must not disappear when logging is turned down.
