# XDG Housekeeper

[![Verify][verify-badge]][verify-workflow]

> Note: while this repository does contain a CLAUDE.md file, tools like Claude
> Code are only used for AI-Assistance and always within a tight feedback loop
> with a human. Code in this repository is never generated in bulk by an agent,
> and never committed without thorough review by a human.

The `xdg-housekeeper` tool checks the `$HOME` directory for any files and
directories spanning configuration, state, data, and caching that instead could
be organized according to the conventions of the [XDG Base Directory
Specification](https://specifications.freedesktop.org/basedir/latest/) by
relocating them into the appropriate XDG directory they belong to.

It computes and presents the full set of proposed changes within the home
directory for inspection. The changes won't take effect until the tool is
invoked with the `apply` command.

## Install

Using Go v1.26.6 or later:

```sh
go install github.com/dragonwasrobot/xdg-housekeeper@latest
```

This places the `xdg-housekeeper` binary in `$GOBIN`, or in `$(go env
GOPATH)/bin` when that is unset, so remember to have it on your `$PATH`.

## Usage

```
xdg-housekeeper tidies the dotfiles in your home directory
into the XDG directories they belong in.

Usage:
  xdg-housekeeper [flags] <command>

Commands:
  help   show this help
  plan   show what would change, without changing anything
  apply  show what would change, then ask before changing it

Flags:
  --home DIR  treat DIR as your home directory instead of $HOME
  --yes       change things without asking first

Flags may be passed on either side of the command.
```

A *dotfile* here is any file or folder directly in your home directory whose
name starts with a dot, such as `.gitconfig` or `.gnupg/`. The *XDG directories*
are where the specification wants them instead: `~/.config`, `~/.local/share`,
`~/.local/state` and `~/.cache`, unless an `XDG_*_HOME` variable points to
somewhere else. The output calls them *base directories*.

Start with `plan`. It lists where your XDG directories are, then what would
happen to each dotfile, and changes nothing. Once that looks right, `apply`
shows the same list, asks before changing anything, and ends by saying what it
did.

To try it on a copy rather than your real home, point `--home` at the copy. That
replaces your home directory and nothing else: the `XDG_*_HOME` variables still
decide where the XDG directories are. Change or unset them too, or `apply` moves
the copy's dotfiles into your real ones:

```sh
xdg-housekeeper --home ~/scratch-home plan
```

## What it changes, and what it won't

- **Nothing is deleted.** A dotfile is only ever renamed into its new location,
  and a `symlink` adds a link back at the old one.
- **Nothing is copied.** When the new location is on a different disk from
  your home directory, that dotfile fails to move rather than being copied
  over and removed.
- **Nothing is overwritten.** A dotfile is skipped as *occupied* when
  something already exists at its new location, and as *blocked* when a file
  sits where a folder on the way there should be. The output names what is in
  the way, and clearing it is up to you.
- **Nothing is guessed.** A dotfile the tool knows but can't move is skipped
  as *unsupported*, with the reason, and one it doesn't know is listed as
  *unmanaged* and left alone.
- **Nothing changes without a yes.** `apply` changes nothing until you answer
  `y` or `yes`, or pass `--yes` up front. Any other answer, or none at all,
  means no.
- **One failure doesn't stop the rest.** A dotfile that fails to move is
  reported under its app once the others are done, and the run exits `1`.

## Example

A home directory with dotfiles from five apps, one of them with a file already
located at its new location, and two dotfiles the tool isn't familiar with:

```
xdg-housekeeper: plan for /home/dwr

base directories
  config  /home/dwr/.config
  data    /home/dwr/.local/share
  state   /home/dwr/.local/state
  cache   /home/dwr/.cache

bash
  skip    .bash_profile: unsupported
          read from a fixed path before any variable applies
  skip    .bashrc: unsupported
          read from a fixed path before any variable applies

git
  move    .gitconfig -> /home/dwr/.config/git/config

gnupg
  symlink .gnupg -> /home/dwr/.local/share/gnupg
  note    set GNUPGHOME to make gpg use the new path directly

less
  symlink .lesshst -> /home/dwr/.local/state/less/history
  note    set LESSHISTFILE to make less use the new path directly

wget
  skip    .wgetrc: occupied
          /home/dwr/.config/wget/wgetrc already exists

unmanaged
  .foorc
  .myapp

3 to relocate, 5 skipped (1 occupied). Nothing has changed.
Run: xdg-housekeeper apply
```

Each app gets a block of its own, with one line per dotfile:

- `move` puts the dotfile in its new location and leaves nothing behind.
- `symlink` does the same, and leaves a link at the old location so the app
  keeps finding it. The `note` below says what to set so the app looks in the
  new location itself, after which the link can go.
- `skip` leaves the dotfile where it is, and says why. The reasons are explained
  [below](#what-it-changes-and-what-it-wont).
- `unmanaged` lists the dotfiles the tool doesn't know, and leaves them alone.

## Exit codes

| Code | Meaning                                                   |
|------|-----------------------------------------------------------|
| 0    | everything was moved or skipped, or you answered no       |
| 1    | something failed partway, so some dotfiles may have moved |
| 2    | the run stopped before changing anything                  |

The run exits with code `2` on a mistyped command or flag, a home directory it
can't use, a new location it can't check, or an answer to the prompt it can't
read. A skip is not counted as a failure. In a script, `2` means your home
directory is as it was, and `1` means it may not be.

## Supported apps

| App   | Dotfile      | Moves to                       | How     | Environment variable(s) to set                   |
|-------|--------------|--------------------------------|---------|--------------------------------------------------|
| aws   | `.aws`       | `$XDG_CONFIG_HOME/aws`         | symlink | `AWS_CONFIG_FILE`, `AWS_SHARED_CREDENTIALS_FILE` |
| git   | `.gitconfig` | `$XDG_CONFIG_HOME/git/config`  | move    |                                                  |
| gnupg | `.gnupg`     | `$XDG_DATA_HOME/gnupg`         | symlink | `GNUPGHOME`                                      |
| less  | `.lesshst`   | `$XDG_STATE_HOME/less/history` | symlink | `LESSHISTFILE`                                   |
| wget  | `.wgetrc`    | `$XDG_CONFIG_HOME/wget/wgetrc` | symlink | `WGETRC`                                         |

`git` already looks in `$XDG_CONFIG_HOME/git` by itself, so its dotfile just
moves. Every other app gets a link at the old location, which keeps it working
until you set the environment variable mentioned in the last column, after which
the link can go. The two `aws` variables each name a file, so point them at
`config` and `credentials` inside the moved folder.

One app can't be moved at all, so its dotfiles are skipped as *unsupported*:

| App  | Dotfiles                   | Reason                                             |
|------|----------------------------|----------------------------------------------------|
| bash | `.bashrc`, `.bash_profile` | read from a fixed path before any variable applies |

## Adding an app

Each supported app is one JSON file in [`apps/`](apps/), built into the
binary. To add one, write a file named after the app's command:

```json
{
  "app": "wget",
  "rules": [
    {
      "entryName": ".wgetrc",
      "location": "config",
      "relPath": "wget/wgetrc",
      "relocation": {
        "kind": "symlink",
        "note": "set WGETRC to make wget use the new path directly"
      }
    }
  ]
}
```

- `entryName` is the dotfile or folder in your home directory.
- `location` is the XDG base directory it belongs in: `config`, `data`,
  `state` or `cache`.
- `relPath` is where it goes inside that directory, and must stay inside it.
- `relocation` is a `move` when the app already reads the new path, and a
  `symlink` otherwise. A symlink can carry a `note` saying how to make the link
  unnecessary, i.e. by setting environment variables; a move can't.

An app whose dotfiles can't be moved lists them under `unsupported` instead,
each with what keeps it in place:

```json
{
  "app": "bash",
  "unsupported": [
    {
      "entryName": ".bashrc",
      "obstacle": "read from a fixed path before any variable applies"
    }
  ]
}
```

Keys match regardless of case, so `EntryName` reads as `entryName`, and a key
given twice keeps its last value. Write each key once, spelled as shown.

Then add the app to the tables above and run `go test ./...`. A file the tool
can't read, a path leading outside its base directory, or a dotfile two apps
claim fails the tests. The tool also refuses to run with any of them, though it
only notices a bad path when that dotfile is in your home directory.

## Prior art

This tool is not unique in trying to keep a clean `$HOME` directory. Other
notable examples are:

- [xdg-ninja](https://github.com/b3nj5m1n/xdg-ninja)
- [antidot](https://github.com/doron-cohen/antidot)

[verify-badge]: https://github.com/dragonwasrobot/xdg-housekeeper/actions/workflows/verify.yml/badge.svg
[verify-workflow]: https://github.com/dragonwasrobot/xdg-housekeeper/actions/workflows/verify.yml
