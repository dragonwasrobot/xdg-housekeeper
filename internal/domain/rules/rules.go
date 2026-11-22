// Package rules defines the rule table: the rules saying where each app's
// entries belong, along with the entries of unsupported apps.
package rules

import (
	"errors"
	"fmt"

	"github.com/samber/lo"

	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/xdg"
)

// Rule captures where one app's entry belongs and how it gets there.
type Rule struct {
	App        string
	EntryName  string
	Location   xdg.Location
	RelPath    string
	Relocation Relocation
}

// Relocation specifies how a rule may relocate an entry it claims.
//
//sumtype:decl
type Relocation interface {
	isRelocation()
}

// RelocationMove moves the entry to its target, leaving nothing behind.
type RelocationMove struct{}

// isRelocation implements Relocation.
func (RelocationMove) isRelocation() {}

// RelocationSymlink moves the entry to its target and leaves a link at the old
// path.
type RelocationSymlink struct {
	// Note details what the user needs to do to make the link redundant.
	Note string
}

// isRelocation implements Relocation.
func (RelocationSymlink) isRelocation() {}

// UnsupportedEntry captures an entry an unsupported app owns, and the
// obstacle that keeps it in place.
type UnsupportedEntry struct {
	App       string
	EntryName string
	Obstacle  string
}

// Table is the rule table: every rule, together with every unsupported entry.
type Table struct {
	rules       []Rule
	unsupported []UnsupportedEntry
}

// Rules returns every rule in the table.
func (t Table) Rules() []Rule { return t.rules }

// UnsupportedEntries returns every unsupported entry in the table.
func (t Table) UnsupportedEntries() []UnsupportedEntry { return t.unsupported }

// ErrDuplicateClaim reports that two rules claim the same entry, which leaves
// the entry with no single place to go.
var ErrDuplicateClaim = errors.New("duplicate claim")

// ErrClaimedUnsupported reports that a rule claims an entry an unsupported app
// owns, which leaves the entry both relocated and kept in place.
var ErrClaimedUnsupported = errors.New("claimed unsupported entry")

// BuildTable builds the rule table from every rule and unsupported entry,
// refusing a set in which two rules claim the same entry, or a rule claiming an
// entry an unsupported app owns.
func BuildTable(rules []Rule, unsupported []UnsupportedEntry) (Table, error) {
	if rule, duplicated := duplicateClaim(rules); duplicated {
		return Table{}, fmt.Errorf("rule for %q: %w",
			rule.EntryName, ErrDuplicateClaim)
	}

	if rule, owned := unsupportedClaim(rules, unsupported); owned {
		return Table{}, fmt.Errorf("rule for %q: %w",
			rule.EntryName, ErrClaimedUnsupported)
	}

	return Table{rules: rules, unsupported: unsupported}, nil
}

// duplicateClaim returns a rule claiming an entry another rule also claims,
// and whether there is one.
func duplicateClaim(rules []Rule) (Rule, bool) {
	duplicates := lo.FindDuplicatesBy(rules, func(rule Rule) string {
		return rule.EntryName
	})

	return lo.First(duplicates)
}

// unsupportedClaim returns a rule claiming an entry an unsupported app owns,
// and whether there is one.
func unsupportedClaim(
	rules []Rule, unsupported []UnsupportedEntry,
) (Rule, bool) {
	owned := lo.KeyBy(unsupported, func(entry UnsupportedEntry) string {
		return entry.EntryName
	})

	return lo.Find(rules, func(rule Rule) bool {
		_, isOwned := owned[rule.EntryName]
		return isOwned
	})
}
