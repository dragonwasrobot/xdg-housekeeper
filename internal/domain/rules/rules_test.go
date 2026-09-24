package rules

import (
	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/xdg"
	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
)

var baseDirs xdg.BaseDirs

var _ = BeforeEach(func() {
	// Given base directories resolved from a home directory
	var err error

	baseDirs, err = xdg.Resolve(xdg.Environment{Home: "/home/dwr"})
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("Rules", func() {
	It("should specify only targets inside the base directories", func() {
		// Given the list of rules
		rules := Rules()

		// When resolving the target of every rule
		for _, rule := range rules {
			_, err := baseDirs.Target(rule.RelPath, rule.Location)

			// Then the rule specifies a target within its base directory
			Expect(err).NotTo(HaveOccurred(), "rule for %q", rule.App)
		}
	})

	It("should claim each entry with at most one rule", func() {
		// Given the list of rules
		rules := Rules()

		// When indexing them by the entry each rule claims
		indexed := lo.KeyBy(rules, func(rule Rule) string {
			return rule.EntryName
		})

		// Then no entry was claimed by more than one rule
		Expect(indexed).To(HaveLen(len(rules)))
	})

	It("should claim no entry an unsupported app owns", func() {
		// Given the list of rules
		claimed := lo.KeyBy(Rules(), func(rule Rule) string {
			return rule.EntryName
		})

		// When checking every rule entry against the unsupported entries
		for _, unsupported := range UnsupportedEntries() {
			// Then no rules claims an unsupported entry
			Expect(claimed).NotTo(HaveKey(unsupported.Name),
				"entry of %q", unsupported.App)
		}
	})
})
