// Package cli is the argument adapter: reading the arguments a run was invoked
// with into the options to be consumed downstream.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
)

// Command is what a run was asked to do.
type Command string

const (
	// CommandHelp asks a run to report how the tool is used.
	CommandHelp Command = "help"

	// CommandPlan asks a run to report its plan, changing nothing on disk.
	CommandPlan Command = "plan"

	// CommandApply asks a run to carry its plan out.
	CommandApply Command = "apply"
)

// Options is everything a run was asked for, as read from its arguments.
type Options struct {
	// Command is what this run carries out, defaulting to CommandHelp.
	Command Command

	// Home overrides home directory. Empty means environment decides.
	Home string

	// Yes skips the confirmation an apply otherwise prompts for.
	Yes bool
}

// Usage reports how the tool is used.
func Usage() string {
	// Written out rather than assembled from the set of flag and commands.
	const usage = `xdg-housekeeper relocates the entries in your home directory
into the XDG base directories they belong in.

Usage:
  xdg-housekeeper [flags] <command>

Commands:
  help   report how the tool is used
  plan   report what would change, leaving every entry where it is
  apply  report what would change, and carry it out

Flags:
  --home DIR  use this directory as home, not the one the environment names
  --yes       skip the confirmation prompt for the apply command

Flags may be passed on either side of the command.
`

	return usage
}

// Parse reads the specified arguments into validated options for downstream
// consumers.
func Parse(args []string) (Options, error) {
	flags, values := newFlagSet()

	commandName, err := parseArgs(args, flags)
	if err != nil {
		// Asking for usage is not a failure, however the flag set reports it.
		if errors.Is(err, flag.ErrHelp) {
			return Options{Command: CommandHelp}, nil
		}

		//nolint:errorlint // flag's error is a stdlib detail
		return Options{}, fmt.Errorf("parse arguments: %v", err)
	}

	command, err := commandFrom(commandName)
	if err != nil {
		return Options{}, err
	}

	if flags.NArg() > 0 {
		return Options{}, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}

	return Options{
		Command: command,
		Home:    values.home,
		Yes:     values.yes,
	}, nil
}

// flagValues holds what each flag was set to. Every field is filled by the
// time Parse reads it: an unnamed flag holds its default.
type flagValues struct {
	home string
	yes  bool
}

// newFlagSet declares the flags a run may be invoked with. The values are
// returned by pointer because the flag set writes into them as it is parsed.
func newFlagSet() (*flag.FlagSet, *flagValues) {
	flags := flag.NewFlagSet("xdg-housekeeper", flag.ContinueOnError)
	// Suppress stderr output for bad flags, use returned error instead.
	flags.SetOutput(io.Discard)

	var values flagValues

	flags.StringVar(&values.home, "home", "",
		"use this directory as home, not the one the environment names")
	flags.BoolVar(&values.yes, "yes", false,
		"skip the confirmation prompt for the apply command")

	return flags, &values
}

// parseArgs reads the specified arguments and returns the command they name,
// leaving the flag set populated with every flag they set.
func parseArgs(args []string, flags *flag.FlagSet) (string, error) {
	if err := flags.Parse(args); err != nil {
		return "", err
	}

	trailing := flags.Args()
	if len(trailing) == 0 {
		return "", nil
	}

	// 2nd pass to support having flags both before and after the command.
	if err := flags.Parse(trailing[1:]); err != nil {
		return "", err
	}

	return trailing[0], nil
}

// commandFrom finds the command matching the argv, treating the absence of one
// as a request for usage.
func commandFrom(name string) (Command, error) {
	if name == "" {
		return CommandHelp, nil
	}

	command := Command(name)

	if !slices.Contains(commands(), command) {
		return "", fmt.Errorf("unknown command %q", name)
	}

	return command, nil
}

// commands returns every command a run may name, in the order Usage lists them.
func commands() []Command {
	return []Command{CommandHelp, CommandPlan, CommandApply}
}
