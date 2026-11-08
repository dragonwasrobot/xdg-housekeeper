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

// Options capture all arguments given to a run.
//
//sumtype:decl
type Options interface {
	isOptions()
}

// Help tells a run to report how the tool is used.
type Help struct{}

func (Help) isOptions() {}

// Housekeeping tells the run to clean up home; spanning both plan and apply.
type Housekeeping struct {
	home         string
	confirmation Confirmation
}

func (Housekeeping) isOptions() {}

// Home returns the optional home directory provided to the run.
func (h Housekeeping) Home() string { return h.home }

// Confirmation returns the run's choice of confirmation.
func (h Housekeeping) Confirmation() Confirmation { return h.confirmation }

// Confirmation specifies where a run's confirmation comes from.
type Confirmation string

const (
	// ConfirmationWithheld is used by plan when the user should not be prompted.
	ConfirmationWithheld Confirmation = "withheld"

	// ConfirmationAsked is used by apply when the user should be prompted.
	ConfirmationAsked Confirmation = "asked"

	// ConfirmationGiven is used by --yes flag when the user trusts the process.
	ConfirmationGiven Confirmation = "given"
)

// command is the first non-flag argument given to a run.
type command string

const (
	commandHelp  command = "help"
	commandPlan  command = "plan"
	commandApply command = "apply"
)

// Usage reports how the tool is used.
func Usage() string {
	// Written out rather than assembled from the set of flag and commands.
	const usage = `xdg-housekeeper tidies the dotfiles in your home directory
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
			return Help{}, nil
		}

		//nolint:errorlint // flag's error is a stdlib detail
		return nil, fmt.Errorf("parse arguments: %v", err)
	}

	command, err := commandFrom(commandName)
	if err != nil {
		return nil, err
	}

	if flags.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}

	yesMisplaced := values.yes && command != commandApply
	if yesMisplaced {
		return nil, fmt.Errorf("--yes is only for the %s command",
			commandApply)
	}

	return optionsFor(command, *values), nil
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
		"treat DIR as your home directory instead of $HOME")
	flags.BoolVar(&values.yes, "yes", false,
		"change things without asking first")

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
func commandFrom(name string) (command, error) {
	if name == "" {
		return commandHelp, nil
	}

	command := command(name)

	if !slices.Contains(commands(), command) {
		return "", fmt.Errorf("unknown command %q", name)
	}

	return command, nil
}

// commands returns every command a run may name, in the order Usage lists them.
func commands() []command {
	return []command{commandHelp, commandPlan, commandApply}
}

// optionsFor returns the options matching the specified command.
func optionsFor(command command, values flagValues) Options {
	switch command {
	case commandHelp:
		return Help{}

	case commandPlan:
		return Housekeeping{home: values.home, confirmation: ConfirmationWithheld}

	case commandApply:
		confirmation := ConfirmationAsked
		if values.yes {
			confirmation = ConfirmationGiven
		}

		return Housekeeping{home: values.home, confirmation: confirmation}

	default:
		panic(fmt.Sprintf("undefined command %q", command))
	}
}
