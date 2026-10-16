# CONTEXT

This document contains a brief introduction to the XDG Base Directory
Specification along with a *domain language*, which lists the vocabulary of XDG
concepts, filesystem terminology, and custom data types used within the
project.

## The XDG Base Directory Specification

The [XDG Base Directory
Specification](https://specifications.freedesktop.org/basedir/latest/) states
where to relocate user-specific application files, such as the dot files that
tend to pile up in the `$HOME` directory. It divides files into four different
types, each with an accompanying environment variable to specify the intended
location:

| Variable          | Default              | Holds                                    |
|-------------------|----------------------|------------------------------------------|
| `XDG_CONFIG_HOME` | `$HOME/.config`      | application configuration                |
| `XDG_DATA_HOME`   | `$HOME/.local/share` | portable data that should be backed up   |
| `XDG_STATE_HOME`  | `$HOME/.local/state` | state that persists but need not be kept |
| `XDG_CACHE_HOME`  | `$HOME/.cache`       | non-essential, regenerable data          |

## Domain Language

The terms of the domain language are grouped relative to the different stages of
a *run* of the `xdg-housekeeper` tool from start to finish.

### Environment

What the tool resolves before it looks in the *home* directory.

- **home**: the user's home directory, taken from the `--home` flag if given and
  from the `HOME` environment variable otherwise, such as `/Users/<username>` on
  macOS or `/home/<username>` on Linux. The specification's defaults are
  resolved against it.
- **location**: the name of one of the four base directories the specification
  defines: config, data, state, or cache. A location resolves to a *base
  directory*, and each *entry* belongs in one of them.
- **XDG variable**: one of the four environment variables the specification
  defines (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`,
  `XDG_CACHE_HOME`), one per *location*.
- **environment**: the raw values of `HOME` and the four *XDG variables*.
- **base directory**: the absolute directory a *location* resolves to. It is the
  path of that location's *XDG variable* if the path is absolute and not the
  filesystem root, and the specification's default otherwise. Every *target*
  lies inside a base directory.

### Invocation

What the user asks the tool to do.

- **run**: one execution of the tool, from the arguments it is given to the exit
  code it returns. Depending on its *command*, a run prints help, shows the
  *plan* it would carry out, or carries out the plan.
- **command**: what a *run* is asked to do, given as the first non-flag
  argument: one of `help`, `plan`, or `apply`, defaulting to `help`.

### Entries

What the tool does in the *home* directory.

- **entry**: a file or directory directly inside *home* whose name starts with a
  dot, such as `.docker/` or `.zsh_history`. A *base directory* is never an
  entry, and neither is a directory that contains one. When using the default
  XDG paths, that leaves out `.config`, `.cache`, and `.local`.
- **traverse**: to scan *home* for *entries*.

### Rules

What the tool knows about each app: where its entries belong, how they get
there, or why they stay put.

- **app**: the program an *entry* belongs to, named after the command that runs
  it, such as `git`, `wget`, or `gnupg`. One app can own several entries.
  `claude` owns both `.claude.json` and `.claude/`, and needs a *rule* for each.
- **rule**: a declaration of where one *app*'s *entry* belongs and how it gets
  there. It gives a *location*, a path inside that location's *base directory*,
  and a *relocation*. For example, `git`'s rule moves `.gitconfig` to
  `git/config` in the config base directory.
- **claim**: a *rule* claims the one *entry* it names, and no entry is claimed
  by more than one rule.
- **relocation**: how a *rule* gets its *entry* into place. A *move* takes the
  entry there and leaves nothing behind. A *symlink* does the same and leaves a
  link at the old path. Each also names the *action* that carries it out.
- **note**: an instruction to the user that comes with a *rule* whose
  *relocation* is a *symlink*, saying how to make the *app* use the new path
  directly. Usually that means setting an environment variable, such as
  `LESSHISTFILE` for `less`. The *report* shows it beside the *action*.
- **unsupported app**: an *app* whose *entries* can't be relocated at all, so no
  *rule* claims them.
- **unsupported entry**: a declaration naming one *entry* an *unsupported app*
  owns, together with the *obstacle* that keeps it in place. It takes the place
  of a *rule*, one per entry, since each entry can have its own obstacle.
- **obstacle**: the reason an *unsupported entry* can't be relocated. `bash`,
  for example, reads `.bashrc` from a fixed path before any variable applies.
  The *report* shows it beside the skipped entry.

### Targets

Where the tool should relocate an entry to, and what might be in the way.

- **target**: the absolute path an *entry* would be relocated to, worked out
  from the *rule* that claims it.
- **invalid target**: the error when a *rule*'s path can't produce a
  *target*, because the path is absolute or leads outside the rule's *base
  directory*. It means the rule table is wrong, so it fails the run instead
  of skipping the entry.
- **probe**: to check the filesystem at a *target* and along its parent
  directories, for anything that would stop an *entry* being relocated
  there. The result is the target's *target state*.
- **target state**: what a *probe* finds at a *target*: occupied, blocked,
  or free. Only an *entry* whose target is free can be relocated.
- **occupied target**: a *target* where something already exists, such as a
  file, a directory, or even a symbolic link that points nowhere.
- **blocked target**: a *target* that can't be created, because a *blocker*
  sits where one of its parent directories should be.
- **blocker**: anything but a directory, or a symbolic link leading to one,
  that a *probe* finds where one of a *blocked target*'s parent directories
  should be. The *report* names it as the thing the user has to move.

### Planning

What the tool will do with each entry.

- **candidate**: a *rule* together with the *entry* it claims and that
  entry's *target*. There is a candidate only for an entry actually in
  *home*.
- **survey**: every *entry* found in *home*, including the ones no rule
  claims, together with the *candidates* built from their *rules*.
- **plan**: one *action* for every *entry* in *home*, all worked out before
  any action is carried out. The same home always gives the same plan:
  actions are sorted by *app* name, then by entry, with the entries no app
  owns last.
- **action**: what the *plan* does with one *entry*. Two actions relocate the
  entry, *move* and *symlink*. The other four skip it, one per *skip reason*.
- **managed**: describes an *action* whose *entry* an *app* owns, which is
  every action except *unmanaged*.
- **skip reason**: why the *plan* leaves an *entry* where it is. Each gives
  its name to the *action* that skips the entry:
  - *blocked*: the entry's *target* is a *blocked target*.
  - *occupied*: the target is an *occupied target*.
  - *unsupported*: an unsupported app owns the entry, and its *obstacle* says
    why it stays.
  - *unmanaged*: no *rule* claims the entry, and no *unsupported app* owns it.

### Applying and reporting

What the tool reports to the user, and what the plan carries out.

- **report**: the text a *run* prints about its *plan*: the *listing*, then the
  *closing count*. An `apply` run prints the listing before carrying out any
  *action*, then a *section* for each *app* whose action failed, then the
  closing count.
- **listing**: the start of every *report*: an opening line naming *home*, then
  the *sections*.
- **section**: one titled block of a *report*, with a heading line saying what
  it covers and indented lines below. A blank line separates sections. In a
  *listing*, the *base directories* come first, then one section per *app*
  holding its *actions* in *plan* order, then a final section for the *entries*
  no app owns.
- **closing count**: the totals at the end of a *report*. For the `plan`
  *command*, it counts *actions*, says that nothing has changed, and names the
  command that carries the plan out if there is anything to relocate. For
  `apply`, it counts *outcomes* alongside the skipped actions, unless the
  *confirmation* was declined. Whichever the count, it lists the *occupied* and
  *blocked* skips separately, since the user has to clear those by hand.
- **outcome**: what happened when the *plan* carried out one *move* or
  *symlink*, the only *actions* that are carried out. Either the *entry* was
  relocated, or the action failed and the entry stayed where it was. A failure
  doesn't stop the *run*, and the *report* shows it in a *section* for its
  *app* after the *listing*.
- **confirmation**: the go-ahead an `apply` *run* asks the user for, after
  showing its *listing* and before carrying out any *action*. Where it comes
  from is what tells the *commands* apart:
  - *withheld*: never asked for and never given, so nothing changes. The
    `plan` command.
  - *asked*: asked of the user once the listing is shown. The `apply` command.
  - *given*: given up front by the `--yes` flag, so the run never asks.
