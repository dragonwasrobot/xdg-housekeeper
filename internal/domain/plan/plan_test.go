package plan

import (
	"testing"

	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/xdg"
	"github.com/google/go-cmp/cmp"
)

func TestCandidates_ShouldResolveTheTargetOfARuleWhoseEntryIsPresent(t *testing.T) {
	// Given base directories, and an entry in home that a rule claims
	baseDirs := defaultBaseDirs(t)

	entries := []Entry{{Path: "/home/dwr/.gitconfig"}}

	// When collecting the candidates
	got, err := Candidates(entries, baseDirs)
	if err != nil {
		t.Fatalf("collect candidates: %v", err)
	}

	// Then the rule that claims it reports the entry and its target
	want := []Candidate{
		{
			Rule: Rule{
				App:        "git",
				EntryName:  ".gitconfig",
				Location:   xdg.LocationConfig,
				RelPath:    "git/config",
				Relocation: RelocationMove,
			},
			Entry:  Entry{Path: "/home/dwr/.gitconfig"},
			Target: "/home/dwr/.config/git/config",
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("candidates mismatch (-want +got):\n%s", diff)
	}
}

func TestCandidates_ShouldLeaveOutAnEntryNoRuleClaims(t *testing.T) {
	// Given base directories, and an entry in home that no rule claims
	baseDirs := defaultBaseDirs(t)

	entries := []Entry{{Path: "/home/dwr/.unclaimed"}}

	// When collecting the candidates
	got, err := Candidates(entries, baseDirs)
	if err != nil {
		t.Fatalf("collect candidates: %v", err)
	}

	// Then there is nothing to relocate
	want := []Candidate{}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("candidates mismatch (-want +got):\n%s", diff)
	}
}

func TestRules_ShouldNameOnlyTargetsInsideTheirBaseDirectories(t *testing.T) {
	// Given base directories
	baseDirs := defaultBaseDirs(t)

	// When resolving the target every rule in the table names
	for _, rule := range rules() {
		_, err := baseDirs.Target(rule.RelPath, rule.Location)

		// Then the rule names one inside its location's base directory
		if err != nil {
			t.Errorf("rule for %q: %v", rule.App, err)
		}
	}
}

func TestRules_ShouldClaimEachEntryWithAtMostOneRule(t *testing.T) {
	// Given the compiled-in rule table
	declared := rules()

	// When indexing it by the entry each rule claims
	indexed := byEntryName(declared)

	// Then no entry was claimed by more than one rule
	if diff := cmp.Diff(len(declared), len(indexed)); diff != "" {
		t.Errorf("claimed entry count mismatch (-want +got):\n%s", diff)
	}
}

func defaultBaseDirs(t *testing.T) xdg.BaseDirs {
	t.Helper()

	baseDirs, err := xdg.Resolve(xdg.Environment{Home: "/home/dwr"})
	if err != nil {
		t.Fatalf("resolve base directories: %v", err)
	}

	return baseDirs
}
