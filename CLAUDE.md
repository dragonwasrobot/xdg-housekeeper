# CLAUDE.md

## Architecture

Single binary, `main.go` at the repo root. Everything else lives under
`internal/`, split into a pure core and an imperative shell:

```
main.go                 package main — flags, exit codes, terminal output
internal/
  domain/               the pure core: no I/O, no env, no clock
    xdg/                package xdg  — XDG base-directory spec knowledge
    plan/               package plan — building a housekeeping plan
  infra/                the imperative shell: the only place the world is touched
    fsys/               package fsys — walking and moving files
    cli/                package cli  — argv parsing
```

Directories carry the **layer**; package names carry the **concern**. So call
sites read `plan.Build(entries, opts)` and `fsys.Walk(root)`, never
`domain.BuildPlan(...)`. Add packages under a layer as concerns appear; never
add a package named for a layer, a pattern, or a catch-all (`util`, `common`,
`helpers`) — if a new package's one-concern sentence is hard to write, it
shouldn't exist yet.

Dependency direction is one-way: **nothing under `internal/infra/` may import
anything under `internal/domain/`.** `main` wires the two together — it calls
`infra` to read the world, hands plain values to `domain`, and calls `infra`
again to write the results back. This is not just convention here: `depguard`
in `.golangci.yml` enforces it on the import path, so a violation fails
`golangci-lint run`.

`internal/` itself is enforced by the Go toolchain — code under it is
importable only from within this module — so these packages carry no
compatibility obligation to anyone outside the repo and can be reshaped freely.

This file has no concrete file references yet because the repo is new. As code
lands, replace the illustrative snippets below with pointers to the real thing
(`see internal/domain/plan/plan.go`) — a principle anchored to a live example
ages better than one anchored to a sketch.

## Guiding principles

These shape how code in this repo is written. They apply to every change.

- **Readability is the most important property.** Optimize for the next
  reader. A clever one-liner that costs five minutes of head-scratching loses
  to the boring version.
- **Let code breathe.** Surround multi-line `if`/`for`/`switch` blocks with a
  blank line above and below, unless they sit immediately under an enclosing
  brace or a comment introducing them. `gofmt` won't do this for you.

  ```go
  // ❌ dense — blocks have no breathing room mid-function
  entry, err := lookupEntry(name)
  if err != nil {
      return Plan{}, fmt.Errorf("look up %q: %w", name, err)
  }
  targets, err := resolveTargets(entry)
  if err != nil {
      return Plan{}, fmt.Errorf("resolve targets for %q: %w", name, err)
  }

  // ✅ blank lines frame each block
  entry, err := lookupEntry(name)
  if err != nil {
      return Plan{}, fmt.Errorf("look up %q: %w", name, err)
  }

  targets, err := resolveTargets(entry)
  if err != nil {
      return Plan{}, fmt.Errorf("resolve targets for %q: %w", name, err)
  }
  ```

  One Go-specific carve-out: an `if err != nil` block belongs to the call above
  it. That pair is a single thought — the blank line goes above the call, never
  between the call and its check.
- **Declaration order is top-down.** If function A calls function B, declare A
  above B — same for unexported helpers. A type goes above the functions that
  build it. The reader sees intent before mechanism.
- **Order parameters by relevance.** The value the function is fundamentally
  about leads; contextual criteria follow; injected config and clients trail:
  `plan.Build(entries, opts)`. A function with no input entity leads with its
  principal criterion. The one exception is `ctx context.Context`, which always
  comes first — that convention outranks this rule, and `revive`'s
  `context-as-argument` rule enforces it.
- **Comments earn their place.** A doc comment says *what* a thing does and
  *why* a caller would reach for it — not *how*; the body shows the how. A
  negation ("does X, not Y") earns its place only when Y is an expectation the
  reader could form from something visible — a sibling function, a stdlib
  default, an adjacent type — not a discarded design idea that only lived in a
  past conversation; the reader can't see the ghost being refuted, so cut it.
- **Every package opens with a doc comment naming its concern.** One to three
  sentences directly above `package x`, phrased `// Package x …`. Move it into
  a `doc.go` once it outgrows that or wants an example. Writing the
  one-concern sentence doubles as a cohesion test — if it's hard to write, the
  package is doing too much. `revive`'s `package-comments` rule enforces it.
- **Every exported identifier gets a doc comment** — functions, types,
  methods, constants, and sentinel errors alike — starting with the
  identifier's own name (`// Build returns …`), per Go convention. Coverage is
  mandatory, not just content: even something that feels like internal plumbing
  gets one. Unexported functions get a doc comment only when they meet the same
  explaining bar; trivial pass-throughs skip it. `revive`'s `exported` rule
  enforces both the coverage and the leading name.
- **Prefer short functions.** 50 LOC is the ceiling, and `funlen` enforces it —
  excluding tests, where a table is mostly rows. Treat approaching it as a
  signal the function is doing several things — find the seams and extract
  named helpers before adding more. The real concern is single responsibility,
  not the line count.
- **The signature is the spec — keep it honest.** No bare `any`/`interface{}`
  crossing a function boundary, and no primitive standing in for a domain
  concept. If a `string` is really a home-relative path or an XDG variable
  name, give it a defined type; if a function takes three `bool`s, it wants an
  options struct.
- **No stringly-typed values for finite variants; branch exhaustively.** Two
  encodings, chosen by whether the variants carry data:

  ```go
  // No payload → defined type + const block.
  type Action string

  const (
      ActionMove   Action = "move"
      ActionDelete Action = "delete"
      ActionSkip   Action = "skip"
  )

  // Differing payloads → sealed interface, closed by an unexported method.
  // The directive is what makes gochecksumtype check switches over it — a
  // sealed interface without it is unchecked.
  //sumtype:decl
  type Outcome interface{ isOutcome() }

  type Moved struct{ From, To string }
  type Skipped struct{ Reason string }

  func (Moved) isOutcome()   {}
  func (Skipped) isOutcome() {}
  ```

  Branch on either with `switch`, never an `if`/`else` chain, and omit
  `default` so that adding a variant breaks every switch that ignores it.
  `exhaustive` enforces that for the typed-constant form and `gochecksumtype`
  for the sealed interface — but only for an interface carrying the
  `//sumtype:decl` directive above it. Without the directive the linter skips
  the type silently, so the encoding looks protected while nothing is checking
  it.
- **Prefer value semantics; don't mutate what you didn't make.** Pass and
  return small structs by value; reach for a pointer when the type is large,
  needs identity, or must genuinely be mutated. Never mutate a slice, map, or
  struct received as an argument — copy first if you need a changed version. No
  exported package-level mutable variables, `var Err…` sentinels excepted —
  `gochecknoglobals` enforces that with exactly this carve-out.
- **Make impossible states unrepresentable.** Model data so a state that can't
  occur can't be constructed — a sealed interface over the variants that
  actually exist, or unexported fields behind a constructor that establishes
  the invariant — rather than an optional field or a loose type that admits
  combinations the rest of the code, and its tests, then have to defend
  against.
- **Prefer a useful zero value; reach for a pointer only when absence is
  real.** Before modelling a field as `*T`, ask whether the absent state can
  actually occur — an optional field that can never be empty is a lie the
  reader has to disprove. When absence exists only at the source, resolve it at
  the boundary so it never enters a domain type. When absence is real *and*
  indistinguishable from the zero value, use `*T` or a `(T, bool)` pair —
  consistently, one convention per concept, never a mix.
- **Functional core, imperative shell.** §Architecture gives the package split;
  this is the rule behind it. Anything computational belongs under
  `internal/domain/`, written as a function of its arguments — it receives
  plain values, never a handle to the world. Impurity is confined to
  `internal/infra/` and the `main` that sequences it, and even there the
  filesystem, clock, and config arrive as arguments rather than being reached
  for through package-level state. `slog` is the deliberate exception: a
  cross-cutting singleton, called directly rather than threaded through
  signatures.
- **Accept interfaces, return structs.** Declare an interface at the *consumer*,
  not beside the implementation, and give it only the methods that consumer
  calls — a one-method interface is the norm, not a degenerate case. Prefer an
  existing stdlib interface to a new one: an `internal/infra/fsys` function
  that reads should take an `io/fs.FS`, which makes `fstest.MapFS` a complete
  test double with no mocking. Return concrete types so callers keep every
  field and method without a type assertion.
- **Compose across package boundaries; embed only within one.** A type holds
  another package's type as a named field rather than embedding it, so a change
  there can't ripple outward through an embedding chain. Embedding is fine
  within a package.

  ```go
  // ❌ every field of xdg.Spec joins Plan's API silently, and renaming one
  // breaks callers of Plan that never mentioned xdg
  type Plan struct {
      xdg.Spec
      Actions []Action
  }

  // ✅ the dependency is visible and bounded
  type Plan struct {
      Spec    xdg.Spec
      Actions []Action
  }

  // ✅ embedding within a package — one author, one change unit
  type moveAction struct {
      baseAction
      From, To string
  }
  ```
- **`defer` cleanup immediately after acquiring.** The `defer f.Close()` goes on
  the line after the successful open, never at the end of the function — the
  pairing stays visible and no early return can skip it. That holds for a file
  you only read. For one you wrote to, `Close` reports flush failures — capture
  and return that error instead of deferring it away.
- **No naked returns.** Name result parameters only when they document an
  otherwise-ambiguous same-typed pair, and return values explicitly even then.
  `nakedret` rejects them in a function of any length.
- **Don't reach for concurrency without a requirement.** No goroutines,
  channels, or mutexes until there is a measured reason. A sequential CLI is
  easier to read, test, and debug — and here concurrent actions touching the
  same parent directory would be a correctness problem, not a speed win.
- **Keep one ubiquitous language across code and docs.** Every domain concept
  has exactly one name, used identically in type names, doc comments, tests,
  and prose. `CONTEXT.md` is the registry — it holds the domain background and
  the definitive term list. Before naming something new, check it; coining a
  genuinely new domain term means adding it to `CONTEXT.md` in the same change.
  Near-synonyms are drift, not style: `entry` and `item` must not name the same
  thing unless both are registered with a defined distinction. When a rename
  settles a conflict, carry it everywhere the term appears — code, tests, docs
  — never let two spellings coexist.
- **Name your conditionals.** Bind a complex boolean to a named local before
  branching on it, or extract a complex predicate into a named function.
  `if isOrphaned(entry)` beats a five-clause boolean.
- **Name your transformation chains.** Four or more successive transformations
  of the same value, inside a function that also does something else, is a
  decomposition signal. Bind the chain to a well-named local when it only
  sequences already-named helpers; extract a named helper when it carries logic
  of its own — new predicates, new error cases, a result worth naming.
- **Alias deep paths.** When a multi-segment access (`a.B.C.X`, `a.B.C.Y`)
  appears more than twice in a function or block, bind the shared prefix to a
  local — the alias becomes the noun the surrounding code is about, and the
  reader stops re-traversing the path. Skip it for a single use; one alias for
  one access just renames the same thing.
- **Validate only at boundaries.** Trust internal callers. Validate at I/O edges
  — flag parsing, environment reads, file contents — and don't re-check an
  already-validated input deeper in the call chain. This does not license
  skipping a check a function needs on its own: one reachable from a sibling
  caller or a unit test handles its own bad input at the point it can fail,
  rather than trusting that validation happened elsewhere.
- **Don't abstract until the second use.** A few similar lines beat a premature
  abstraction, so the first occurrence stays as it is. Extract when duplication
  actually appears — at the second call site, never in anticipation of one that
  hasn't landed.
- **Return `(T, error)` for expected failures; `panic` only for the
  unrecoverable.**
  - A function with an expected failure mode — a missing directory, a malformed
    config line, a target that escapes `$HOME` — returns `(T, error)`.
  - Declare sentinels at package level: `var ErrEscapingTarget =
    errors.New("target escapes home directory")`. Error strings are lowercase
    and unpunctuated so they read correctly once wrapped into a longer chain —
    `staticcheck`'s ST1005 catches the trailing punctuation, the lowercase
    opening is on you. `errname` enforces the naming: sentinels are `ErrXxx`,
    error types are `XxxError`.
  - Wrap with context at every layer that has some to add:
    `fmt.Errorf("resolve %q: %w", name, err)`. Use `%w`, never `%v` — `%v`
    flattens the error to text and breaks `errors.Is` for every caller
    upstream. `errorlint` catches it.
  - Callers branch with `errors.Is` for identity and `errors.As` when they need
    data out of the failure; a failure carrying data wants its own error type.
    Never compare `err.Error()` strings. `errorlint` catches `==` against a
    sentinel; the string comparison it won't, so that one is on you.
  - Handle every error where it is produced. Never `_ = err` — `errcheck` runs
    with `check-blank`, so the blank assignment fails the same as an ignored
    return. Never put several fallible calls under one handler that loses which
    of them failed.
  - Return it or log it, never both — see §Logging conventions.
  - `panic` is for violated invariants and programmer error, never for bad
    input.
  - `main` is the dispatcher: it unwraps the chain, writes a message to stderr,
    and exits non-zero. It is the only place that chooses an exit code.
  - Avoid an `error`-only return for a function worth calling. An operation that
    did something can usually report what — a status variant, a count of entries
    moved, the resulting path.

  ```go
  // internal/domain/xdg
  var ErrEscapingTarget = errors.New("target escapes home directory")

  func Resolve(entry Entry, home string) (Target, error) {
      target, err := join(home, entry.Path)
      if err != nil {
          return Target{}, fmt.Errorf("resolve %q: %w", entry.Name, err)
      }

      if !within(home, target) {
          return Target{}, fmt.Errorf("resolve %q: %w", entry.Name, ErrEscapingTarget)
      }

      return target, nil
  }

  // caller — identity survives the wrapping
  target, err := xdg.Resolve(entry, home)
  if errors.Is(err, xdg.ErrEscapingTarget) {
      slog.Warn("skipping entry", "name", entry.Name)
      continue
  }
  ```

## Development workflow

Work proceeds in tight cycles, with the developer reading the diff at the end
of each one. The skills below hold the detail — numbered steps, verification
commands, worked examples, what each loop forbids. What is stated here is only
what binds *before* a skill loads.

- **New behavior → the `feature` skill.** Development is test-first (**Red →
  Green → Blue**): **never write production code without a failing test
  motivating it**, and never start the next cycle in the same response as the
  current one.
- **Restructuring existing code → the `refactor` skill.** **Apply one labeled
  change, then stop and hand the diff back** — sequential edits in one response
  are still a batch, even as separate tool calls.
- **Writing or changing a test → the `testing` skill.** Load it before writing
  the test, not to check one after.

## Logging conventions

- **`log/slog` is for diagnostics; `fmt.Fprintln(os.Stdout, …)` is for
  CLI-user-facing text; `fmt.Fprintln(os.Stderr, …)` is for the dispatcher's
  error output.** That last split is the one that matters: the message printed
  when a run fails is this tool's answer to the person who ran it, not a
  diagnostic for an observability backend — so it never goes through `slog`.
  Never `log.Printf`, and never `fmt.Println` for a diagnostic — `forbidigo`
  bans the stdlib `log` package and the unqualified `fmt.Print*` calls outright.
- **Four levels.** `debug` for argument and computed-value dumps; `info` for
  milestones; `warn` for an anomaly the run survives, such as an unreadable
  entry that gets skipped; `error` exactly once, at the dispatcher.
  `internal/domain` and `internal/infra` return errors — they do not log them.
  A failure logged where it happens *and* returned gets reported twice, and the
  duplicate is the one without the surrounding context.
- **Message shape.** Use the structured key/value form, not interpolation: a
  constant human-readable message, then the dynamic values as attributes. The
  message is what you grep for; the attributes are what you filter on.
  `sloglint` enforces both halves — `kv-only` for the attribute form,
  `static-msg` for the constant message.

  ```go
  slog.Debug("parsed arguments", "args", args)
  slog.Info("moved entry", "from", src, "to", dst)
  slog.Warn("skipping unreadable entry", "path", path, "err", err)

  fmt.Fprintf(os.Stderr, "xdg-housekeeper: %v\n", err)
  ```
- **A dry run is output, not logging.** What the tool would do goes to stdout as
  user-facing text, at every verbosity — it is the answer to the question the
  user asked, so it must not disappear when logging is turned down.
