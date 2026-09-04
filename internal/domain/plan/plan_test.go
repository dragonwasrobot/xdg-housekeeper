package plan

import (
	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/xdg"
	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
)

// Every spec below resolves its targets against the same home directory, so the
// suite builds it once per spec rather than each container repeating itself.
var baseDirs xdg.BaseDirs

var _ = BeforeEach(func() {
	// Given base directories resolved from a home directory
	var err error

	baseDirs, err = xdg.Resolve(xdg.Environment{Home: "/home/dwr"})
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("Candidates", func() {
	Context("when a rule claims an entry in home", func() {
		It("should report the entry and the target it resolves to", func() {
			// Given an entry in home that a rule claims
			entries := []Entry{{Path: "/home/dwr/.gitconfig"}}

			// When collecting the candidates
			candidates, err := Candidates(entries, baseDirs)

			// Then the rule that claims it reports the entry and its target
			Expect(err).NotTo(HaveOccurred())
			Expect(candidates).To(BeComparableTo([]Candidate{
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
			}))
		})
	})

	Context("when no rule claims an entry in home", func() {
		It("should leave that entry out", func() {
			// Given an entry in home that no rule claims
			entries := []Entry{{Path: "/home/dwr/.unclaimed"}}

			// When collecting the candidates
			candidates, err := Candidates(entries, baseDirs)

			// Then there is nothing to relocate
			Expect(err).NotTo(HaveOccurred())
			Expect(candidates).To(BeComparableTo([]Candidate{}))
		})
	})
})

var _ = Describe("Rules", func() {
	It("should name only targets inside their base directories", func() {
		// Given the compiled-in rule table
		declared := rules()

		// When resolving the target every rule in the table names
		for _, rule := range declared {
			_, err := baseDirs.Target(rule.RelPath, rule.Location)

			// Then the rule names one inside its location's base directory
			Expect(err).NotTo(HaveOccurred(), "rule for %q", rule.App)
		}
	})

	It("should claim each entry with at most one rule", func() {
		// Given the compiled-in rule table
		declared := rules()

		// When indexing it by the entry each rule claims
		indexed := byEntryName(declared)

		// Then no entry was claimed by more than one rule
		Expect(indexed).To(HaveLen(len(declared)))
	})
})
