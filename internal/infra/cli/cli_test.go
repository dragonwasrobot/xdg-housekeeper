package cli

import (
	"flag"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2/dsl/core"
	"github.com/onsi/ginkgo/v2/dsl/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Usage", func() {
	It("should document every command a run may be invoked with", func() {
		// When reporting how the tool is used
		usage := Usage()

		// Then each command is listed in the usage description
		for _, command := range commands() {
			commandLine := "\n  " + string(command) + " "

			Expect(usage).To(ContainSubstring(commandLine))
		}
	})

	It("should document every flag a run may be invoked with", func() {
		// Given the flags a run is actually parsed against
		flags, _ := newFlagSet()

		// When reporting how the tool is used
		usage := Usage()

		// Then each flag is listed in the usage description
		flags.VisitAll(func(f *flag.Flag) {
			Expect(usage).To(ContainSubstring("--" + f.Name))
			Expect(usage).To(ContainSubstring(f.Usage))
		})
	})
})

var _ = Describe("Parse", func() {
	table.DescribeTable("should report how the tool is used",
		func(args []string) {
			// When reading those arguments
			options, err := Parse(args)

			// Then the run says how it is used
			Expect(err).NotTo(HaveOccurred())
			Expect(options).To(BeComparableTo(Help{}))
		},
		table.Entry("because a run named no command at all",
			[]string(nil)),
		table.Entry("because a run named the help command",
			[]string{"help"}),
		table.Entry("because a run included the help flag",
			[]string{"--help"}),
	)

	Context("when a run includes flags before the command", func() {
		table.DescribeTable("should report everything the run was invoked with",
			func(args []string, want Options) {
				// When reading those arguments
				options, err := Parse(args)

				// Then the run handles both flags and command
				Expect(err).NotTo(HaveOccurred())
				Expect(options).To(BeComparableTo(want,
					cmp.AllowUnexported(Housekeeping{})))
			},
			table.Entry("because a run named the plan command",
				[]string{"--home", "/home/dwr", "plan"},
				Housekeeping{
					home:         "/home/dwr",
					confirmation: ConfirmationWithheld,
				}),
			table.Entry("because a run named the apply command",
				[]string{"--home", "/home/dwr", "apply"},
				Housekeeping{
					home:         "/home/dwr",
					confirmation: ConfirmationAsked,
				}),
			table.Entry("because a run named the apply command and its flags",
				[]string{"--home", "/home/dwr", "--yes", "apply"},
				Housekeeping{
					home:         "/home/dwr",
					confirmation: ConfirmationGiven,
				}),
		)
	})

	Context("when a run is invoked with a flag after its command", func() {
		It("should read it as though it came before", func() {
			// Given a flag on the far side of the command it applies to
			args := []string{"apply", "--yes"}

			// When reading those arguments
			options, err := Parse(args)

			// Then the run handles them similar to prefixed flags
			Expect(err).NotTo(HaveOccurred())
			Expect(options).To(BeComparableTo(Housekeeping{
				confirmation: ConfirmationGiven,
			}, cmp.AllowUnexported(Housekeeping{})))
		})
	})

	table.DescribeTable("should refuse to start",
		func(args []string) {
			// When reading those arguments
			options, err := Parse(args)

			// Then the run never starts
			Expect(err).To(HaveOccurred())
			Expect(options).To(BeNil())
		},
		table.Entry("because no part of the tool declares that command",
			[]string{"bogus"}),
		table.Entry("because no part of the tool declares that flag",
			[]string{"--bogus"}),
		table.Entry("because nothing takes the argument after the command",
			[]string{"apply", "extra"}),
		table.Entry("because only the apply command asks for a confirmation",
			[]string{"plan", "--yes"}),
	)
})
