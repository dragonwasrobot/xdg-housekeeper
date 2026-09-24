// Package plan turns the entries found in home into a housekeeping plan: which
// entries can be relocated, how should they be relocated, which are skipped.
package plan

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/samber/lo"

	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/rules"
	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/xdg"
)

// Entry is one dot-prefixed file or directory existing in home. Path is
// absolute; obtained from traversing the home directory.
type Entry struct {
	Path string
}

// Name returns the entry's name in home, which a rule uses to claim it.
func (e Entry) Name() string {
	return filepath.Base(e.Path)
}

// Candidate pairs a rule with the entry it claims and its intended target.
// A candidate only exists for a rule whose entry is present in home.
type Candidate struct {
	Rule   rules.Rule
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
	claimedBy := lo.KeyBy(rules.Rules(), func(rule rules.Rule) string {
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

// Plan is the ordered set of actions covering every entry found in home.
type Plan []Action

// Action is what a plan emits for one entry: either relocating the entry to its
// target, or skipping it and leaving it where it is.
//
//sumtype:decl
type Action interface {
	isAction()

	// EntryPath returns the path in home of the entry this action covers.
	EntryPath() string
}

// Managed is an action whose entry an app owns.
//
//sumtype:decl
type Managed interface {
	Action

	// App returns the app that owns the entry.
	App() string
}

// Relocation is an action that moves its entry to a target.
//
//sumtype:decl
type Relocation interface {
	Action

	// Target returns the path the entry relocates to.
	Target() xdg.Target
}

// Move relocates the entry to its target, leaving nothing behind.
type Move struct {
	app       string
	entryPath string
	target    xdg.Target
}

// isAction implements Action.
func (Move) isAction() {}

// App implements Managed.
func (m Move) App() string { return m.app }

// EntryPath implements Action.
func (m Move) EntryPath() string { return m.entryPath }

// Target implements Relocation.
func (m Move) Target() xdg.Target { return m.target }

// Symlink relocates the entry to its target and leaves a symlink at the old
// path, so an app that cannot be pointed at the new location keeps working.
type Symlink struct {
	app       string
	entryPath string
	target    xdg.Target
	note      string
}

// isAction implements Action.
func (Symlink) isAction() {}

// App implements Managed.
func (s Symlink) App() string { return s.app }

// EntryPath implements Action. It is also where the symlink is left behind.
func (s Symlink) EntryPath() string { return s.entryPath }

// Target implements Relocation.
func (s Symlink) Target() xdg.Target { return s.target }

// Note returns what the user has to do to make the symlink redundant.
func (s Symlink) Note() (string, bool) { return s.note, s.note != "" }

// Occupied skips the entry, because something already exists at its target.
type Occupied struct {
	app       string
	entryPath string
	target    xdg.Target
}

// isAction implements Action.
func (Occupied) isAction() {}

// App implements Managed.
func (o Occupied) App() string { return o.app }

// EntryPath implements Action.
func (o Occupied) EntryPath() string { return o.entryPath }

// OccupiedTarget returns the target something already sits at.
func (o Occupied) OccupiedTarget() xdg.Target { return o.target }

// Blocked skips the entry, because a non-directory exists on the path where the
// target's parent directories need to be.
type Blocked struct {
	app       string
	entryPath string
	blocker   string
}

// isAction implements Action.
func (Blocked) isAction() {}

// App implements Managed.
func (b Blocked) App() string { return b.app }

// EntryPath implements Action.
func (b Blocked) EntryPath() string { return b.entryPath }

// Blocker returns the file standing between the entry and its target.
func (b Blocked) Blocker() string { return b.blocker }

// Unsupported captures an entry that cannot be relocated.
type Unsupported struct {
	app       string
	entryPath string
	obstacle  string
}

// isAction implements Action.
func (Unsupported) isAction() {}

// App implements Managed. An unsupported app still claims its entries.
func (u Unsupported) App() string { return u.app }

// EntryPath implements Action.
func (u Unsupported) EntryPath() string { return u.entryPath }

// Obstacle returns what stands in the way of relocating the app at all.
func (u Unsupported) Obstacle() string { return u.obstacle }

// Unmanaged skips the entry, because no rule claims it.
type Unmanaged struct {
	entryPath string
}

// isAction implements Action.
func (Unmanaged) isAction() {}

// EntryPath implements Action.
func (u Unmanaged) EntryPath() string { return u.entryPath }

// TargetState is what a probe found standing between an entry and its target.
//
//sumtype:decl
type TargetState interface {
	isTargetState()
}

// TargetOccupied reports that something already exists at the target.
type TargetOccupied struct{}

// isTargetState implements TargetState.
func (TargetOccupied) isTargetState() {}

// TargetBlocked reports that a non-directory exists where one of the target's
// parent directories would have to be, with Blocker naming that file.
type TargetBlocked struct {
	Blocker string
}

// isTargetState implements TargetState.
func (TargetBlocked) isTargetState() {}

// Build derives the action every entry in the survey calls for, using the
// probed states to tell the entries with a free target from the ones the plan
// has to skip.
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

// checkRelocationSkip returns the action that skips the entry when the probe
// state stops it from relocating. Returns nil otherwise.
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
			blocker:   state.Blocker,
		}, true
	}

	return nil, false
}

// relocationFor returns the action for the relocation the candidate's rule
// declares. It panics on a rule declaring no relocation.
func relocationFor(candidate Candidate) Action {
	rule := candidate.Rule
	entryPath := candidate.Entry.Path

	switch relocation := rule.Relocation.(type) {
	case rules.RelocationMove:
		return Move{
			app:       rule.App,
			entryPath: entryPath,
			target:    candidate.Target,
		}

	case rules.RelocationSymlink:
		return Symlink{
			app:       rule.App,
			entryPath: entryPath,
			target:    candidate.Target,
			note:      relocation.Note,
		}
	}

	panic(fmt.Sprintf("rule for %q declares no relocation", rule.App))
}

// skips derives the action for every unclaimed entry.
func skips(survey Survey) []Action {
	claimedPaths := lo.KeyBy(survey.candidates, func(candidate Candidate) string {
		return candidate.Entry.Path
	})

	unsupportedEntries := lo.KeyBy(rules.UnsupportedEntries(),
		func(entry rules.UnsupportedEntry) string {
			return entry.Name
		})

	skipFor := func(entry Entry, _ int) (Action, bool) {
		return checkUnclaimedSkip(entry, claimedPaths, unsupportedEntries)
	}

	return lo.FilterMap(survey.entries, skipFor)
}

// checkUnclaimedSkip returns the action that skips an entry no rule claims, and
// whether no rule claims it.
func checkUnclaimedSkip(entry Entry,
	claimedPaths map[string]Candidate,
	unsupportedEntries map[string]rules.UnsupportedEntry) (Action, bool) {
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

// ordered deterministically sorts the plan, first by the app that owns an
// action, then by entry, with the unmanaged entries placed at the end.
func ordered(actions []Action) Plan {
	managed := ManagedActions(actions)
	unmanaged := lo.Reject(actions, ownedByApp)

	slices.SortStableFunc(managed, byAppThenEntry)
	slices.SortStableFunc(unmanaged, byEntry)

	asAction := func(action Managed, _ int) Action { return action }

	return append(lo.Map(managed, asAction), unmanaged...)
}

// ManagedActions returns the actions an app owns, which is every action but
// Unmanaged.
func ManagedActions(actions []Action) []Managed {
	return lo.FilterMap(actions, asManaged)
}

// ownedByApp reports whether an app owns the entry the action covers.
func ownedByApp(action Action, index int) bool {
	_, owned := asManaged(action, index)

	return owned
}

// asManaged returns the action as a Managed, and whether an app owns it.
func asManaged(action Action, _ int) (Managed, bool) {
	managed, owned := action.(Managed)

	return managed, owned
}

// byAppThenEntry orders two actions by the app that owns them, breaking a tie
// on the entry so an app owning several entries sorts the same way every run.
func byAppThenEntry(a Managed, b Managed) int {
	if byApp := strings.Compare(a.App(), b.App()); byApp != 0 {
		return byApp
	}

	return byEntry(a, b)
}

// byEntry orders two actions by the entry they cover.
func byEntry(a Action, b Action) int {
	return strings.Compare(a.EntryPath(), b.EntryPath())
}
