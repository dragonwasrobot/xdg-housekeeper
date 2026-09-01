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

## Prior art

This tool is not unique in trying to keep a clean `$HOME` directory. Other
notable examples are:

- [xdg-ninja](https://github.com/b3nj5m1n/xdg-ninja)
- [antidot](https://github.com/doron-cohen/antidot)

[verify-badge]: https://github.com/dragonwasrobot/xdg-housekeeper/actions/workflows/verify.yml/badge.svg
[verify-workflow]: https://github.com/dragonwasrobot/xdg-housekeeper/actions/workflows/verify.yml
