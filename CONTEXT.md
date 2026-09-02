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
- **claim**: what a *rule* does to the one *entry* it names. Every rule claims
  exactly one entry, and every entry is claimed by at most one rule; an entry no
  rule claims is *unmanaged*.
- **target**: the resolved absolute destination path of an *entry*, derived from
  the *rule* matching the *entry*.
- **invalid target**: the failure of a *rule* to name a *target* at all, because
  the path it gives within the *base directory* is absolute, or resolves outside
  that base directory, or its *location* is not one of the four the
  specification defines. Always produces a failure, never a warning.
- **occupied target**: a *target* that already exists in its *base directory*,
  leaving the *entry* nowhere to relocate to. Always produces a skip, and is the
  one *skip reason* that also warrants a warning, since it is the only one the
  user has to decide something about.
- **relocation**: how a *rule* relocates its *entry*: either by moving it
  outright, or by moving it and leaving a *symlink* behind. The rule declares
  it, the *plan* carries it out; a *move* rule yields a *move* *action*.
- **symlink**: the symbolic link between the original *entry* path and the new
  entry path relocated within the appropriate *location*. Used when an *app*
  does not fully adhere to the XDG specification, and the old entry path still
  needs to be present. Also names the *relocation* and the *action* that create
  one.
- **candidate**: a *rule* paired with a matching *entry* in *home*, along with
  the *target* that entry should relocate to. A *plan* is built from one or
  more candidates.
- **action**: what the *plan* emits for one *entry*, one of five: *move* and
  *symlink* relocate it; *occupied*, *unmanaged*, and *unsupported* skip it,
  each named for its *skip reason*.
- **skip reason**: why the *plan* skips an *entry* instead of relocating it. One
  of three: *occupied*, the entry's *target* is an *occupied target*;
  *unmanaged*, no *rule* *claims* the entry; *unsupported*, the entry's *app*
  has no rule because it cannot be relocated at all; it reads the entry from a
  fixed path and offers no way to point it elsewhere.
- **plan**: the complete ordered set of *actions*, computed before any of them
  run, based on the *entries* in *home* and the declared *rules*. A plan is a
  value; nothing changes on disk until the plan is applied.
