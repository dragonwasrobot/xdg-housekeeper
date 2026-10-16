// Package report renders the plan a run derived, and the outcomes of running
// it, as terminal text.
package report

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/samber/lo"

	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/plan"
	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/xdg"
)

// Plan renders the report for a 'plan' command: its listing, the count, and a
// remark saying that nothing has changed.
func Plan(actions plan.Plan, home string, baseDirs xdg.BaseDirs) string {
	opening := fmt.Sprintf("xdg-housekeeper: plan for %s\n", home)

	return listing(actions, baseDirs, opening) + "\n" + planSummary(actions)
}

// Applying renders the report for an 'apply' command before it runs: its
// listing, but no count as it hasn't applied anything yet.
func Applying(actions plan.Plan, home string, baseDirs xdg.BaseDirs) string {
	opening := fmt.Sprintf("xdg-housekeeper: applying to %s\n", home)

	return listing(actions, baseDirs, opening)
}

// Outcomes renders what an apply run closes with once the user has read its
// listings and confirmed its plan: only failing apps and the closing count.
func Outcomes(actions plan.Plan, failed map[plan.Relocation]error) string {
	sections := failureSections(actions, failed)
	sections = append(sections, applySummary(actions, failed))

	return "\n" + strings.Join(sections, "\n")
}

// Declined renders what an apply run closes with once the user has read its
// listing and declined the confirmation.
func Declined(actions plan.Plan) string {
	return "\n" + planCount(actions)
}

// listing renders the part every report shares: the specified opening, where
// every base directory resolved, and the action every entry calls for.
func listing(actions plan.Plan, baseDirs xdg.BaseDirs, opening string) string {
	sections := []string{opening, baseDirsSection(baseDirs)}

	sections = append(sections,
		actionSections(plan.ManagedActions(actions))...)

	asUnmanaged := func(action plan.Action, _ int) (plan.Unmanaged, bool) {
		unmanaged, unowned := action.(plan.Unmanaged)
		return unmanaged, unowned
	}

	sections = append(sections,
		unmanagedSection(lo.FilterMap(actions, asUnmanaged))...)

	return strings.Join(sections, "\n")
}

// baseDirsSection renders where every location resolved.
func baseDirsSection(baseDirs xdg.BaseDirs) string {
	line := func(location xdg.Location, _ int) string {
		return fmt.Sprintf("%-8s%s", location, baseDirs.BaseDir(location))
	}

	return section("base directories", lo.Map(xdg.Locations(), line))
}

// actionSections renders what a listing holds for the actions an app owns:
// the lines each action contributes.
func actionSections(actions []plan.Managed) []string {
	return appSections(actions, func(action plan.Managed, _ int) []string {
		return actionLines(action)
	})
}

// failureSections renders only the apps whose actions failed, each holding the
// failures of its actions.
func failureSections(
	actions plan.Plan,
	failed map[plan.Relocation]error) []string {
	didFail := func(action plan.Action, _ int) bool {
		return failureOf(action, failed) != nil
	}

	failedActions := lo.Filter(actions, didFail)

	failureLine := func(err error) string {
		return fmt.Sprintf("%-8s%v", "failed", err)
	}

	return appSections(plan.ManagedActions(failedActions),
		func(action plan.Managed, _ int) []string {
			return []string{failureLine(failureOf(action, failed))}
		})
}

// failureOf returns the error causing the action to fail, or nil if none.
func failureOf(action plan.Action, failed map[plan.Relocation]error) error {
	relocation, relocates := action.(plan.Relocation)
	if !relocates {
		return nil
	}

	return failed[relocation]
}

// appSections renders one section per app owning an action, headed by the app's
// name and holding the specified lines of each action it owns.
func appSections(
	actions []plan.Managed,
	linesFor func(action plan.Managed, index int) []string,
) []string {
	owningApp := func(action plan.Managed) string { return action.App() }

	appSection := func(owned []plan.Managed, _ int) string {
		return section(owned[0].App(), lo.FlatMap(owned, linesFor))
	}

	return lo.Map(lo.PartitionBy(actions, owningApp), appSection)
}

// unmanagedSection renders the entries no rule claims, in a section of their
// own because they name no app to sort under.
func unmanagedSection(actions []plan.Unmanaged) []string {
	if len(actions) == 0 {
		return nil
	}

	line := func(action plan.Unmanaged, _ int) string {
		return filepath.Base(action.EntryPath())
	}

	return []string{section("unmanaged", lo.Map(actions, line))}
}

// section renders one block of a report.
func section(heading string, lines []string) string {
	body := lo.Map(lines, func(line string, _ int) string {
		return fmt.Sprintf("  %s\n", line)
	})

	return heading + "\n" + strings.Join(body, "")
}

// actionLines renders the lines of one action under an app's section.
func actionLines(action plan.Managed) []string {
	switch action := action.(type) {
	case plan.Move:
		return []string{relocationLine(action, "move")}

	case plan.Symlink:
		lines := []string{relocationLine(action, "symlink")}

		if note, noted := action.Note(); noted {
			lines = append(lines, fmt.Sprintf("%-8s%s", "note", note))
		}

		return lines

	case plan.Occupied:
		return skipLines(action.EntryPath(), "occupied",
			fmt.Sprintf("%s already exists", action.OccupiedTarget()))

	case plan.Unsupported:
		return skipLines(action.EntryPath(), "unsupported",
			action.Obstacle())

	case plan.Blocked:
		return skipLines(action.EntryPath(), "blocked",
			fmt.Sprintf("%s exists and is not a directory", action.Blocker()))
	}

	panic(fmt.Sprintf("undefined action %T", action))
}

// relocationLine renders a relocating action's verb and details.
func relocationLine(action plan.Relocation, verb string) string {
	return fmt.Sprintf("%-8s%s -> %s",
		verb, filepath.Base(action.EntryPath()), action.Target())
}

// skipLines renders a skipped entry's reason along with any details.
func skipLines(entryPath string, reason string, detail string) []string {
	return []string{
		fmt.Sprintf("%-8s%s: %s", "skip", filepath.Base(entryPath), reason),
		fmt.Sprintf("%-8s%s", "", detail),
	}
}

// applySummary renders the count and action outcomes of an apply run.
func applySummary(actions plan.Plan, failed map[plan.Relocation]error) string {
	relocatingActions := lo.CountBy(actions, relocating)

	summary := fmt.Sprintf("%d relocated, %d skipped%s",
		relocatingActions-len(failed),
		len(actions)-relocatingActions,
		skipBreakdown(actions))

	if len(failed) > 0 {
		summary += fmt.Sprintf(", %d failed", len(failed))
	}

	return summary + ".\n"
}

// planSummary renders the closing count of a plan run.
func planSummary(actions plan.Plan) string {
	summary := planCount(actions)

	if lo.CountBy(actions, relocating) == 0 {
		return summary
	}

	return summary + "Run: xdg-housekeeper apply\n"
}

// planCount renders how many actions a plan would relocate and skip.
func planCount(actions plan.Plan) string {
	relocations := lo.CountBy(actions, relocating)

	return fmt.Sprintf("%d to relocate, %d skipped%s. Nothing has changed.\n",
		relocations, len(actions)-relocations, skipBreakdown(actions))
}

// skipBreakdown renders the counts of the skip reasons the user has to act on.
func skipBreakdown(actions plan.Plan) string {
	counted := []string{}

	if occupied := lo.CountBy(actions, isOccupied); occupied > 0 {
		counted = append(counted, fmt.Sprintf("%d occupied", occupied))
	}

	if blocked := lo.CountBy(actions, isBlocked); blocked > 0 {
		counted = append(counted, fmt.Sprintf("%d blocked", blocked))
	}

	if len(counted) == 0 {
		return ""
	}

	return fmt.Sprintf(" (%s)", strings.Join(counted, ", "))
}

func isOccupied(action plan.Action) bool {
	_, occupied := action.(plan.Occupied)

	return occupied
}

func isBlocked(action plan.Action) bool {
	_, blocked := action.(plan.Blocked)

	return blocked
}

func relocating(action plan.Action) bool {
	_, relocates := action.(plan.Relocation)

	return relocates
}
