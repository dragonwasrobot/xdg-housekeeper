// Package main relocates the entries in the home directory into the XDG base
// directories they belong in.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/samber/lo"

	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/plan"
	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/xdg"
	"github.com/dragonwasrobot/xdg-housekeeper/internal/infra/cli"
	"github.com/dragonwasrobot/xdg-housekeeper/internal/infra/fsys"
	"github.com/dragonwasrobot/xdg-housekeeper/internal/infra/report"
)

const (
	// exitSuccess reports that nothing the run attempted failed, including a
	// run that stopped because the answer declined its plan.
	exitSuccess = 0

	// exitFailure reports that the run failed after it began carrying out its
	// plan, whether an action failed or its outcomes could not be reported.
	exitFailure = 1

	// exitNotStarted reports that the run failed before it began carrying out
	// its plan, so nothing on disk was touched at all.
	exitNotStarted = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run carries out one run of the tool, from the arguments it was invoked with
// through to the exit code it returns, reporting to the specified streams.
func run(
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) int {
	options, err := cli.Parse(args)
	if err != nil {
		errMessage := errorLine(err) + "\n" + cli.Usage()
		return reportFailure(stderr, errMessage, exitNotStarted)
	}

	var applied bool

	switch options := options.(type) {
	case cli.Help:
		err = writeOut(stdout, cli.Usage())

	case cli.Housekeeping:
		applied, err = runHousekeeping(options, stdin, stdout)
	}

	if err != nil {
		return reportFailure(stderr, errorLine(err), exitCodeFor(applied))
	}

	return exitSuccess
}

// exitCodeFor returns the code a run ends on when it could not fully succeed.
func exitCodeFor(applied bool) int {
	if applied {
		return exitFailure
	}

	return exitNotStarted
}

// reportFailure presents the user with an error message and exit code.
func reportFailure(stderr io.Writer, message string, code int) int {
	if err := writeOut(stderr, message); err != nil {
		slog.Warn("could not report the failure", "err", err)
	}

	return code
}

// errorLine renders the one line the dispatcher names a failure on.
func errorLine(err error) string {
	return fmt.Sprintf("xdg-housekeeper: %v\n", err)
}

// runHousekeeping cleans up home, reporting/applying the plan. It reports the
// outcome of the run and any failures.
func runHousekeeping(
	options cli.Housekeeping,
	stdin io.Reader,
	stdout io.Writer,
) (bool, error) {
	home := homePath(options.Home())

	switch confirmation := options.Confirmation(); confirmation {
	case cli.ConfirmationWithheld:
		return false, runPlan(home, stdout)

	case cli.ConfirmationAsked, cli.ConfirmationGiven:
		return runApply(home, confirmation, stdin, stdout)

	default:
		panic(fmt.Sprintf("undefined confirmation %q", confirmation))
	}
}

// runPlan reports the plan the run would carry out, changing nothing on disk.
func runPlan(home string, stdout io.Writer) error {
	actions, baseDirs, err := planFor(home)
	if err != nil {
		return err
	}

	return writeOut(stdout, report.Plan(actions, home, baseDirs))
}

// actionsFailedError reports the number of failed actions of a plan.
type actionsFailedError struct {
	count int
}

// Error implements error.
func (e actionsFailedError) Error() string {
	if e.count == 1 {
		return "1 action failed"
	}

	return fmt.Sprintf("%d actions failed", e.count)
}

// runApply reports the plan, carries it out and reports what came of it. It
// reports whether it got as far as carrying the plan out.
func runApply(
	home string,
	confirmation cli.Confirmation,
	stdin io.Reader,
	stdout io.Writer,
) (bool, error) {
	actions, baseDirs, err := planFor(home)
	if err != nil {
		return false, err
	}

	listing := report.Applying(actions, home, baseDirs)
	if err := writeOut(stdout, listing); err != nil {
		return false, err
	}

	if confirmation == cli.ConfirmationAsked {
		confirmed, err := confirm(stdin, stdout)
		if err != nil {
			return false, err
		}

		if !confirmed {
			return false, writeOut(stdout, report.Declined(actions))
		}
	}

	failed := applyActions(actions)

	if err := writeOut(stdout, report.Outcomes(actions, failed)); err != nil {
		return true, err
	}

	if len(failed) > 0 {
		return true, actionsFailedError{count: len(failed)}
	}

	return true, nil
}

// confirm asks whether to carry out the plan, and reports the answer.
func confirm(stdin io.Reader, stdout io.Writer) (bool, error) {
	if err := writeOut(stdout, "\nContinue? [y/N] "); err != nil {
		return false, err
	}

	answer, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read confirmation: %w", err)
	}

	answer = strings.ToLower(strings.TrimSpace(answer))

	return answer == "y" || answer == "yes", nil
}

// planFor derives a plan for the entries in the home directory, along with the
// base directories every target was resolved against.
func planFor(home string) (plan.Plan, xdg.BaseDirs, error) {
	baseDirs, err := xdg.Resolve(environment(home))
	if err != nil {
		return nil, xdg.BaseDirs{}, err
	}

	actions, err := buildPlan(home, baseDirs)
	if err != nil {
		return nil, xdg.BaseDirs{}, err
	}

	return actions, baseDirs, nil
}

// applyActions carries out every relocation the plan holds, leaving the
// entries it skips where they are, and reports what went wrong for each
// relocation that failed.
//
// A failed action doesn't stop the process half as relocations are independent.
func applyActions(actions plan.Plan) map[plan.Relocation]error {
	asRelocation := func(action plan.Action, _ int) (plan.Relocation, bool) {
		relocation, relocates := action.(plan.Relocation)
		return relocation, relocates
	}

	failed := map[plan.Relocation]error{}

	for _, relocation := range lo.FilterMap(actions, asRelocation) {
		if err := applyAction(relocation); err != nil {
			failed[relocation] = err
		}
	}

	return failed
}

// applyAction carries out one relocation.
func applyAction(action plan.Relocation) error {
	switch action := action.(type) {
	case plan.Move:
		return fsys.Move(action.EntryPath(), action.Target())

	case plan.Symlink:
		return fsys.Symlink(action.EntryPath(), action.Target())
	}

	panic(fmt.Sprintf("undefined relocation %T", action))
}

// homePath returns the home directory for the run.
func homePath(override string) string {
	if override != "" {
		return override
	}

	return os.Getenv("HOME")
}

// environment reads the raw values every base directory is resolved from.
func environment(home string) xdg.Environment {
	return xdg.Environment{
		Home:       home,
		ConfigHome: os.Getenv("XDG_CONFIG_HOME"),
		DataHome:   os.Getenv("XDG_DATA_HOME"),
		StateHome:  os.Getenv("XDG_STATE_HOME"),
		CacheHome:  os.Getenv("XDG_CACHE_HOME"),
	}
}

// buildPlan surveys the entries in home and derives the action each one calls
// for, probing what stands in the way of their targets in between.
func buildPlan(home string, baseDirs xdg.BaseDirs) (plan.Plan, error) {
	entries, err := fsys.Traverse(home, baseDirs)
	if err != nil {
		return nil, err
	}

	survey, err := plan.Claim(entries, baseDirs)
	if err != nil {
		return nil, err
	}

	states, err := fsys.Probe(survey.Targets())
	if err != nil {
		return nil, err
	}

	return plan.Build(survey, states), nil
}

// writeOut writes user-facing text to the stream a run reports on.
func writeOut(w io.Writer, text string) error {
	if _, err := io.WriteString(w, text); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	return nil
}
