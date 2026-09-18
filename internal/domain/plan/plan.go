// Package plan turns the entries found in home into a housekeeping plan: which
// rule claims each entry, where it belongs, and what should happen to it.
package plan

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/samber/lo"

	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/xdg"
)

// Entry is one dot-prefixed file or directory sitting directly in home. Path is
// absolute; obtained from traversing the home directory.
type Entry struct {
	Path string
}

// Name returns the entry's name in home, which a rule uses to claim it.
func (e Entry) Name() string {
	return filepath.Base(e.Path)
}

// Relocation names how a rule relocates the entry it claims.
type Relocation string

const (
	// RelocationMove moves the entry to its target, leaving nothing behind.
	RelocationMove Relocation = "move"
	// RelocationSymlink moves the entry to its target and leaves a symlink at
	// the old path, so an app that cannot be pointed at the new location keeps
	// working.
	RelocationSymlink Relocation = "symlink"
)

// Rule maps an entry in home to the location and target it belongs in.
type Rule struct {
	App        string
	EntryName  string
	Location   xdg.Location
	RelPath    string
	Relocation Relocation
}

// Candidate pairs a rule with the entry it claims and its intended target.
// A candidate only exists for a rule whose entry is present in home.
type Candidate struct {
	Rule   Rule
	Entry  Entry
	Target xdg.Target
}

// Survey is every entry found in home together with the candidates its rules
// produced.
type Survey struct {
	entries    []Entry
	candidates []Candidate
}

// Targets returns the target every candidate resolved to.
func (s Survey) Targets() []xdg.Target {
	return lo.Map(s.candidates, func(candidate Candidate, _ int) xdg.Target {
		return candidate.Target
	})
}

// Claim surveys the entries in home, pairing each with the candidate its rule
// produced. An entry no rule claims yields no candidate but stays in the
// survey, since the plan still has to account for it.
func Claim(entries []Entry, baseDirs xdg.BaseDirs) (Survey, error) {
	claimedBy := lo.KeyBy(rules(), func(rule Rule) string {
		return rule.EntryName
	})
	candidates := make([]Candidate, 0, len(entries))

	for _, entry := range entries {
		rule, claimed := claimedBy[entry.Name()]

		if !claimed {
			continue
		}

		targetPath, err := baseDirs.Target(rule.RelPath, rule.Location)
		if err != nil {
			return Survey{}, fmt.Errorf("resolve target for %q: %w",
				rule.App, err)
		}

		candidates = append(candidates, Candidate{
			Rule:   rule,
			Entry:  entry,
			Target: targetPath,
		})
	}

	return Survey{entries: entries, candidates: candidates}, nil
}

// rules returns the hardcoded rules list, which declares one rule per entry it
// claims.
func rules() []Rule {
	return []Rule{
		{
			App:        "git",
			EntryName:  ".gitconfig",
			Location:   xdg.LocationConfig,
			RelPath:    "git/config",
			Relocation: RelocationMove,
		},
	}
}

// Action is what a plan emits for one entry: either relocating the entry to its
// target, or skipping it and leaving it where it is.
//
//sumtype:decl
type Action interface {
	// Sealed interface in combination with sumtype:decl
	isAction()

	// App returns the optional app that owns the entry.
	App() (string, bool)

	// EntryPath returns the path in home of the entry this action covers.
	EntryPath() string
}

// Move relocates the entry to its target, leaving nothing behind.
type Move struct {
	app       string
	entryPath string
	target    xdg.Target
}

// isAction implements Action.
func (Move) isAction() {}

// App implements Action.
func (m Move) App() (string, bool) { return m.app, true }

// EntryPath implements Action.
func (m Move) EntryPath() string { return m.entryPath }

// Target returns the path the entry relocates to.
func (m Move) Target() xdg.Target { return m.target }

// Symlink relocates the entry to its target and leaves a symlink at the old
// path, so an app that cannot be pointed at the new location keeps working.
type Symlink struct {
	app       string
	entryPath string
	target    xdg.Target
}

// isAction implements Action.
func (Symlink) isAction() {}

// App implements Action.
func (s Symlink) App() (string, bool) { return s.app, true }

// EntryPath implements Action. It is also where the symlink is left behind.
func (s Symlink) EntryPath() string { return s.entryPath }

// Target returns the path the entry relocates to.
func (s Symlink) Target() xdg.Target { return s.target }

// Occupied skips the entry, because something already exists at its target.
type Occupied struct {
	app       string
	entryPath string
	target    xdg.Target
}

// isAction implements Action.
func (Occupied) isAction() {}

// App implements Action.
func (o Occupied) App() (string, bool) { return o.app, true }

// EntryPath implements Action.
func (o Occupied) EntryPath() string { return o.entryPath }

// Target returns the occupied path, which is what the user has to act on.
func (o Occupied) Target() xdg.Target { return o.target }

// Blocked skips the entry, because a regular file exists on the path where the
// target's parent directories need to be. The obstacle specifies the file.
type Blocked struct {
	app       string
	entryPath string
	obstacle  string
}

// isAction implements Action.
func (Blocked) isAction() {}

// App implements Action.
func (b Blocked) App() (string, bool) { return b.app, true }

// EntryPath implements Action.
func (b Blocked) EntryPath() string { return b.entryPath }

// Obstacle returns what stands between the entry and its target.
func (b Blocked) Obstacle() string { return b.obstacle }

// Unsupported skips the entry, because its app cannot be relocated at all. The
// obstacle says what stands in the way, since no rule will ever claim it.
type Unsupported struct {
	app       string
	entryPath string
	obstacle  string
}

// isAction implements Action.
func (Unsupported) isAction() {}

// App implements Action. An unsupported app still owns its entries despite no
// rule claiming them.
func (u Unsupported) App() (string, bool) { return u.app, true }

// EntryPath implements Action.
func (u Unsupported) EntryPath() string { return u.entryPath }

// Obstacle returns what stands in the way of relocating the app at all.
func (u Unsupported) Obstacle() string { return u.obstacle }

// Unmanaged skips the entry, because no rule claims it. It names no app, since
// nothing here knows which app the entry belongs to.
type Unmanaged struct {
	entryPath string
}

// isAction implements Action.
func (Unmanaged) isAction() {}

// App implements Action, reporting false: no rule claims the entry and no app
// is known to own it.
func (u Unmanaged) App() (string, bool) { return "", false }

// EntryPath implements Action.
func (u Unmanaged) EntryPath() string { return u.entryPath }

// Plan is the ordered set of actions covering every entry found in home,
// computed in full before any of them run. A plan is a value; nothing changes
// on disk until it is applied.
type Plan []Action

// TargetState is what a probe found standing between an entry and its target.
//
//sumtype:decl
type TargetState interface {
	// Sealed interface in combination with sumtype:decl
	isTargetState()
}

// TargetOccupied reports that something already exists at the target.
type TargetOccupied struct{}

// isTargetState implements TargetState.
func (TargetOccupied) isTargetState() {}

// TargetBlocked reports that a regular file exists where one of the target's
// parent directories would have to be, with Blocker naming that file.
type TargetBlocked struct {
	Blocker string
}

// isTargetState implements TargetState.
func (TargetBlocked) isTargetState() {}

// Build derives the action every entry in the survey calls for, using the
// probed states to tell the entries with a free target from the ones the plan
// has to skip. It panics on a rule declaring an unknown relocation.
func Build(survey Survey, states map[xdg.Target]TargetState) Plan {
	actions := make([]Action, 0, len(survey.entries))

	actions = append(actions, relocations(survey, states)...)
	actions = append(actions, skips(survey)...)

	return ordered(actions)
}

// relocations derives the action for every entry covered by a candidate.
func relocations(survey Survey, states map[xdg.Target]TargetState) []Action {
	actionFor := func(candidate Candidate, _ int) Action {
		if skip, skipped := checkRelocationSkip(candidate, states); skipped {
			return skip
		}

		return relocationFor(candidate)
	}

	return lo.Map(survey.candidates, actionFor)
}

// checkRelocationSkip returns the action that skips the entry when the state
// probed at its target stops it from relocating, and whether a state does.
func checkRelocationSkip(
	candidate Candidate, states map[xdg.Target]TargetState,
) (Action, bool) {
	rule := candidate.Rule
	entryPath := candidate.Entry.Path

	switch state := states[candidate.Target].(type) {
	case TargetOccupied:
		return Occupied{
			app:       rule.App,
			entryPath: entryPath,
			target:    candidate.Target,
		}, true

	case TargetBlocked:
		return Blocked{
			app:       rule.App,
			entryPath: entryPath,
			obstacle: fmt.Sprintf("%s already exists and is not a directory",
				state.Blocker),
		}, true
	}

	return nil, false
}

// relocationFor returns the relocation the candidate's rule declares. It panics
// on a rule declaring an unknown relocation.
func relocationFor(candidate Candidate) Action {
	rule := candidate.Rule
	entryPath := candidate.Entry.Path

	switch rule.Relocation {
	case RelocationMove:
		return Move{
			app:       rule.App,
			entryPath: entryPath,
			target:    candidate.Target,
		}

	case RelocationSymlink:
		return Symlink{
			app:       rule.App,
			entryPath: entryPath,
			target:    candidate.Target,
		}

	default:
		panic(fmt.Sprintf("rule for %q declares unknown relocation %q",
			rule.App, rule.Relocation))
	}
}

// skips derives the action for every unclaimed entry.
func skips(survey Survey) []Action {
	claimedPaths := lo.KeyBy(survey.candidates, func(candidate Candidate) string {
		return candidate.Entry.Path
	})

	unsupportedEntries := lo.KeyBy(unsupportedApps(),
		func(app unsupportedApp) string {
			return app.EntryName
		})

	skipFor := func(entry Entry, _ int) (Action, bool) {
		return checkUnclaimedSkip(entry, claimedPaths, unsupportedEntries)
	}

	return lo.FilterMap(survey.entries, skipFor)
}

// checkUnclaimedSkip returns the action that skips an entry no rule claims,
// and whether no rule claims it.
func checkUnclaimedSkip(entry Entry,
	claimedPaths map[string]Candidate,
	unsupportedEntries map[string]unsupportedApp) (Action, bool) {
	if _, isClaimed := claimedPaths[entry.Path]; isClaimed {
		return nil, false
	}

	unsupported, named := unsupportedEntries[entry.Name()]
	if !named {
		return Unmanaged{entryPath: entry.Path}, true
	}

	return Unsupported{
		app:       unsupported.App,
		entryPath: entry.Path,
		obstacle:  unsupported.Obstacle,
	}, true
}

// unsupportedApp names one entry of an app that cannot be relocated at all,
// and what stands in the way. These apps have no rule, since there is no
// target to relocate them to.
type unsupportedApp struct {
	App       string
	EntryName string
	Obstacle  string
}

// unsupportedApps returns the hardcoded list of apps that cannot be relocated.
func unsupportedApps() []unsupportedApp {
	return []unsupportedApp{
		{
			App:       "bash",
			EntryName: ".bashrc",
			Obstacle:  "read from a fixed path before any variable applies",
		},
	}
}

// ordered deterministically sorts the plan, first by actions app name, then by
// entry, with unmanaged apps placed at the end of the plan.
func ordered(actions []Action) Plan {
	hasApp := func(action Action, _ int) bool {
		_, owned := action.App()

		return owned
	}

	owned := lo.Filter(actions, hasApp)
	unowned := lo.Reject(actions, hasApp)

	slices.SortStableFunc(owned, byAppThenEntry)
	slices.SortStableFunc(unowned, byEntry)

	return append(owned, unowned...)
}

// byAppThenEntry orders two actions by the app that owns them, breaking a tie
// on the entry so an app owning several entries sorts the same way every run.
func byAppThenEntry(a Action, b Action) int {
	appA, _ := a.App()
	appB, _ := b.App()

	if byApp := strings.Compare(appA, appB); byApp != 0 {
		return byApp
	}

	return byEntry(a, b)
}

// byEntry orders two actions by the entry they cover.
func byEntry(a Action, b Action) int {
	return strings.Compare(a.EntryPath(), b.EntryPath())
}
