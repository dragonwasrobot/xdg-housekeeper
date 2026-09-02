// Package plan turns the entries found in home into a housekeeping plan: which
// rule claims each entry, where it belongs, and what should happen to it.
package plan

import (
	"fmt"
	"path/filepath"

	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/xdg"
)

// Entry is one dot-prefixed file or directory sitting directly in home. Path is
// absolute; obtained from walking the home directory. Everything else about an
// entry follows from it.
type Entry struct {
	Path string
}

// Name returns the entry's name in home, which a rule uses to claims it.
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

// Candidates returns one candidate per entry a rule claims, carrying the target
// that rule resolved to. An entry no rule claims is left out.
func Candidates(entries []Entry, baseDirs xdg.BaseDirs) ([]Candidate, error) {
	claimedBy := byEntryName(rules())
	candidates := make([]Candidate, 0, len(entries))

	for _, entry := range entries {
		rule, claimed := claimedBy[entry.Name()]

		if !claimed {
			continue
		}

		targetPath, err := baseDirs.Target(rule.RelPath, rule.Location)
		if err != nil {
			return nil, fmt.Errorf("resolve target for %q: %w", rule.App, err)
		}

		candidates = append(candidates, Candidate{
			Rule:   rule,
			Entry:  entry,
			Target: targetPath,
		})
	}

	return candidates, nil
}

// byEntryName indexes the rule table by the entry each rule claims, which is
// how an entry found in home finds the rule claiming it.
func byEntryName(table []Rule) map[string]Rule {
	claimedBy := make(map[string]Rule, len(table))

	for _, rule := range table {
		claimedBy[rule.EntryName] = rule
	}

	return claimedBy
}

// rules returns the hardcoded rule table, which declares one rule per entry it
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
