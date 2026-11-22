package rules

import (
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"

	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/xdg"
)

var _ = Describe("BuildTable", func() {
	Context("when each entry is named only once", func() {
		It("should hold every rule and unsupported entry", func() {
			// Given a rule and an unsupported entry naming different entries
			rules := []Rule{gitRule()}
			unsupported := []UnsupportedEntry{
				{
					App:       "bash",
					EntryName: ".bashrc",
					Obstacle:  "read from a fixed path before any variable applies",
				},
			}

			// When building the rule table
			table, err := BuildTable(rules, unsupported)

			// Then the table holds both declarations
			Expect(err).NotTo(HaveOccurred())
			Expect(table).To(BeComparableTo(
				Table{rules: rules, unsupported: unsupported},
				cmp.AllowUnexported(Table{}),
			))
		})
	})

	Context("when two rules claim the same entry", func() {
		It("should refuse to build the table", func() {
			// Given two rules claiming one entry for different targets
			claimer := gitRule()
			rival := gitRule()
			rival.App = "git-lfs"
			rival.RelPath = "git-lfs/config"

			// When building the rule table
			_, err := BuildTable([]Rule{claimer, rival}, nil)

			// Then the table is refused for claiming the entry twice
			Expect(err).To(MatchError(ErrDuplicateClaim))
		})
	})

	Context("when a rule claims an entry an unsupported app owns", func() {
		It("should refuse to build the table", func() {
			// Given a rule and an unsupported entry naming the same entry
			claimer := gitRule()
			unsupported := UnsupportedEntry{
				App:       "git",
				EntryName: ".gitconfig",
				Obstacle:  "read from a fixed path before any variable applies",
			}

			// When building the rule table
			_, err := BuildTable([]Rule{claimer},
				[]UnsupportedEntry{unsupported})

			// Then the table is refused for claiming an unsupported entry
			Expect(err).To(MatchError(ErrClaimedUnsupported))
		})
	})
})

// gitRule returns a rule claiming .gitconfig, which relocates by moving.
func gitRule() Rule {
	return Rule{
		App:        "git",
		EntryName:  ".gitconfig",
		Location:   xdg.LocationConfig,
		RelPath:    "git/config",
		Relocation: RelocationMove{},
	}
}
