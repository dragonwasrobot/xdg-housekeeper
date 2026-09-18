# CONTEXT

Domain background and the definitive domain language for this project. Every
term below has exactly one name, used identically in type declarations, doc
comments, tests, and prose.

## The XDG Base Directory Specification

The [XDG Base Directory
Specification](https://specifications.freedesktop.org/basedir/latest/) lays out
conventions for where to place various categories of user-specific files, such
as those that usually end up as dot files in your `HOME` directory. Each
category of file has an associated environment variable and a fallback value
used when that variable is unset or empty:

| Variable          | Fallback             | Category                                 |
|-------------------|----------------------|------------------------------------------|
| `XDG_CONFIG_HOME` | `$HOME/.config`      | application configuration                |
| `XDG_DATA_HOME`   | `$HOME/.local/share` | portable data that should be backed up   |
| `XDG_STATE_HOME`  | `$HOME/.local/state` | state that persists but need not be kept |
| `XDG_CACHE_HOME`  | `$HOME/.cache`       | non-essential, regenerable data          |

In order to adhere to the specification, the following constraints apply:

- A variable set to a relative path must be treated as unset.
- A variable set to a path outside of `$HOME` is valid, i.e. setting
  `XDG_DATA_HOME=/mnt/data` is perfectly legal.

While the specification is native to Linux, being a
[freedesktop.org](https://www.freedesktop.org/wiki/) standard, it also has a lot
of traction among apps on macOS, despite the platform's own idiomatic locations
(`~/Library/Application Support`, `~/Library/Caches`) for these kinds of data.

## Domain language

The following terms are used consistently across functions, types, comments and
documentation to define the domain language for the `xdg-housekeeper` tool:

- **home**: the user's home directory. The root everything is resolved against.
  Defined by the `HOME` environment variable and usually of the form
  `/Users/<username>` or similar.
- **entry**: a dot-prefixed file or directory sitting directly in `HOME`, e.g.
  `.docker/` or `.zsh_history`.
- **traverse**: the act of scanning the *home* directory for *entries*, and the
  only way they act as input to the tool. The scan is one level deep and filters
  only dot-prefixed entries; an XDG *base directory* is never itself an entry,
  nor is a directory holding one below it, as `.local` does with the data and
  state base directories at their defaults.
- **XDG variable**: one of the four environment variables the specification
  defines: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`,
  `XDG_CACHE_HOME`. Each variable specifies the *base directory* of one
  *location*.
- **environment**: the raw values every *base directory* is resolved from:
  `HOME`, which gives *home*, plus the four *XDG variables*. An empty value
  stands for a variable that is not set.
- **location**: the XDG *base directory* an *entry* belongs in: config, data,
  state, or cache.
- **base directory**: the absolute directory one *location* resolves to. This is
  either the path specified by the *XDG variable*, or the specification's
  default when that variable is unset, empty, or specifies a relative path. Any
  valid *target* will always live inside a base directory.
- **rule**: the *app*-specific mapping from an *entry* to the *location* it
  belongs in and its path within that location's *base directory*, along with
  its *relocation*.
- **app**: the program or tool an *entry* belongs to, named according to its CLI
  invocation: `git`, `wget`, `gnupg`. One app can own more than one *entry*,
  e.g. the `claude` app may have a *rule* for each of the entries `.claude.json`
  and `.claude/`.
- **unsupported app**: an *app* whose *entries* cannot be relocated at all, so
  no *rule* ever claims them. Declared in its own list beside the *rules*, one
  element per *entry* the app owns, each element specifying an *obstacle*.
- **obstacle**: what stands in the way of relocating an *entry*, specified in
  the *action* so the *plan* says why it skipped it. An *unsupported app*
  declares its own; a *blocked target* renders its *blocker* into one.
- **claim**: what a *rule* does to the one *entry* it names. Every rule claims
  exactly one entry, and every entry is claimed by at most one rule; an entry no
  rule claims is *unsupported* when an *unsupported app* owns it, and
  *unmanaged* otherwise.
- **target**: the resolved absolute destination path of an *entry*, derived from
  the *rule* matching the *entry*.
- **invalid target**: the failure of a *rule* to name a *target* at all, because
  the path it gives within the *base directory* is absolute, or resolves outside
  that base directory. Always produces a failure, never a warning. A *location*
  outside the four the specification defines is not one of these: the four are
  compiled in, so naming another is programmer error, and it panics.
- **probe**: the act of checking the filesystem for anything that stands between
  an entry and its *target*, which may result in either an *occupied target* or
  a *blocked target*. A target is free when nothing was found.
- **occupied target**: a *target* where a file already exists in its *base
  directory* location, leaving the *entry* nowhere to relocate to. Always
  produces a skip, and is one of the two *skip reasons* the user has to act on,
  with a *blocked target* being the other, so the *plan*'s summary spells it out
  separately.
- **blocked target**: a *target* nothing can be placed at, because a
  non-directory exists where one of its parent directories would have to be.
  Usually a regular file, though a socket, a device node, or a *symlink*
  pointing at nothing also acts a blocks. Nothing exists at the target itself,
  so it is not an *occupied target*; the *blocker* is the file in the way.
- **blocker**: the file a *probe* found standing in the way of a *target*'s
  parent chain, captured by the *blocked target*.
- **relocation**: how a *rule* relocates its *entry*: either by moving it
  outright, or by moving it and leaving a *symlink* behind. The rule declares
  it, the *plan* carries it out; a *move* rule yields a *move* *action*.
- **symlink**: the symbolic link between the original *entry* path and the new
  entry path relocated within the appropriate *location*. Used when an *app*
  does not fully adhere to the XDG specification, and the old entry path still
  needs to be present. Also names the *relocation* and the *action* that create
  one.
- **candidate**: a *rule* paired with a matching *entry* in *home*, along with
  the *target* that entry should relocate to. Only an entry present in home
  produces one.
- **survey**: every *entry* found in *home* together with the *candidates* its
  *rules* produced. The value a *plan* is built from, so a candidate can never
  name an entry the survey does not hold.
- **action**: what the *plan* emits for one *entry*, one of six: *move* and
  *symlink* relocate it; *blocked*, *occupied*, *unmanaged*, and *unsupported*
  skip it, each named for its *skip reason*.
- **skip reason**: why the *plan* skips an *entry* instead of relocating it. One
  of four: *blocked*, the entry's *target* is a *blocked target*; *occupied*,
  the entry's *target* is an *occupied target*; *unmanaged*, no *rule* *claims*
  the entry and no *unsupported app* owns it; *unsupported*, an *unsupported
  app* owns the entry, and its *obstacle* specifies what stands in the way.
- **plan**: the complete set of *actions*, computed before any of them run,
  based on the *entries* in *home* and the declared *rules*. Ordered by *app*
  name, with the *entries* no app owns trailing every app, so the same *home*
  always yields the same plan. A plan is a value; nothing changes on disk until
  the plan is applied.
