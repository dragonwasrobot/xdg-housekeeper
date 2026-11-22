package report

import (
	"errors"

	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/plan"
	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/rules"
	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/xdg"
)

var baseDirs xdg.BaseDirs

var _ = BeforeEach(func() {
	var err error

	baseDirs, err = xdg.Resolve(xdg.Environment{Home: "/home/dwr"})
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("Plan", func() {
	Context("when a move rule claims the entry", func() {
		It("should report the target it relocates to", func() {
			// Given one entry in home, and nothing standing in its way
			actions := actionsFor(
				[]plan.Entry{{Path: "/home/dwr/.gitconfig"}}, nil)

			// When rendering the plan
			report := Plan(actions, "/home/dwr", baseDirs)

			// Then the run is told what the base directories resolved to, and
			// what the one action would do when applied
			Expect(report).To(BeComparableTo(planHeader(false) +
				`git
  move    .gitconfig -> /home/dwr/.config/git/config

1 to relocate, 0 skipped. Nothing has changed.
Run: xdg-housekeeper apply
`))
		})
	})

	Context("when a symlink rule claims the entry", func() {
		It("should report the note its rule declares", func() {
			// Given one entry a symlink rule claims
			actions := actionsFor(
				[]plan.Entry{{Path: "/home/dwr/.lesshst"}}, nil)

			// When rendering the plan
			report := Plan(actions, "/home/dwr", baseDirs)

			// Then the note follows the action line
			Expect(report).To(BeComparableTo(planHeader(false) +
				`less
  symlink .lesshst -> /home/dwr/.local/state/less/history
  note    set LESSHISTFILE to make less use the new path directly

1 to relocate, 0 skipped. Nothing has changed.
Run: xdg-housekeeper apply
`))
		})
	})

	Context("when something already exists at the entry's target", func() {
		It("should skip it and name the target in the way", func() {
			// Given the probe found the entry's target occupied
			actions := actionsFor(
				[]plan.Entry{{Path: "/home/dwr/.wgetrc"}},
				map[xdg.Target]plan.TargetState{
					"/home/dwr/.config/wget/wgetrc": plan.TargetOccupied{},
				})

			// When rendering the plan
			report := Plan(actions, "/home/dwr", baseDirs)

			// Then the summary notes this both in the action line and in the
			// final count
			Expect(report).To(BeComparableTo(planHeader(false) +
				`wget
  skip    .wgetrc: occupied
          /home/dwr/.config/wget/wgetrc already exists

0 to relocate, 1 skipped (1 occupied). Nothing has changed.
`))
		})
	})

	Context("when a file exists in the target's parent chain", func() {
		It("should skip it and name the file standing in the way", func() {
			// Given the probe found the entry's target blocked
			actions := actionsFor(
				[]plan.Entry{{Path: "/home/dwr/.gitconfig"}},
				map[xdg.Target]plan.TargetState{
					"/home/dwr/.config/git/config": plan.TargetBlocked{
						Blocker: "/home/dwr/.config/git",
					},
				})

			// When rendering the plan
			report := Plan(actions, "/home/dwr", baseDirs)

			// Then the blocker is named in the action line
			Expect(report).To(BeComparableTo(planHeader(false) +
				`git
  skip    .gitconfig: blocked
          /home/dwr/.config/git exists and is not a directory

0 to relocate, 1 skipped (1 blocked). Nothing has changed.
`))
		})
	})

	Context("when one entry's target is occupied and another's blocked", func() {
		It("should count each reason the user has to act on", func() {
			// Given the probe found one of each type standing in the way
			actions := actionsFor(
				[]plan.Entry{
					{Path: "/home/dwr/.gitconfig"},
					{Path: "/home/dwr/.wgetrc"},
				},
				map[xdg.Target]plan.TargetState{
					"/home/dwr/.config/wget/wgetrc": plan.TargetOccupied{},
					"/home/dwr/.config/git/config": plan.TargetBlocked{
						Blocker: "/home/dwr/.config/git",
					},
				})

			// When rendering the plan
			report := Plan(actions, "/home/dwr", baseDirs)

			// Then both counts are spelled out, in one parenthesis
			Expect(report).To(BeComparableTo(planHeader(false) +
				`git
  skip    .gitconfig: blocked
          /home/dwr/.config/git exists and is not a directory

wget
  skip    .wgetrc: occupied
          /home/dwr/.config/wget/wgetrc already exists

0 to relocate, 2 skipped (1 occupied, 1 blocked). Nothing has changed.
`))
		})
	})

	Context("when an unsupported app owns the entries", func() {
		It("should skip each under the one heading they share", func() {
			// Given both entries of an app that cannot be relocated at all
			actions := actionsFor([]plan.Entry{
				{Path: "/home/dwr/.bash_profile"},
				{Path: "/home/dwr/.bashrc"},
			}, nil)

			// When rendering the plan
			report := Plan(actions, "/home/dwr", baseDirs)

			// Then each specifies its own obstacle in the plan
			Expect(report).To(BeComparableTo(planHeader(false) +
				`bash
  skip    .bash_profile: unsupported
          read from a fixed path before any variable applies
  skip    .bashrc: unsupported
          read from a fixed path before any variable applies

0 to relocate, 2 skipped. Nothing has changed.
`))
		})
	})

	Context("when no rule claims the entries", func() {
		It("should list them in a section of their own", func() {
			// Given entries nothing claims
			actions := actionsFor([]plan.Entry{
				{Path: "/home/dwr/.foorc"},
				{Path: "/home/dwr/.myapp"},
			}, nil)

			// When rendering the plan
			report := Plan(actions, "/home/dwr", baseDirs)

			// Then they are all listed in the unmanaged section
			Expect(report).To(BeComparableTo(planHeader(false) +
				`unmanaged
  .foorc
  .myapp

0 to relocate, 2 skipped. Nothing has changed.
`))
		})
	})

	Context("when every case is present at once", func() {
		It("should give each its own section, with the unmanaged ones last", func() {
			// Given an entry for each kind of section the plan can hold
			actions := actionsFor([]plan.Entry{
				{Path: "/home/dwr/.bash_profile"},
				{Path: "/home/dwr/.bashrc"},
				{Path: "/home/dwr/.foorc"},
				{Path: "/home/dwr/.gitconfig"},
				{Path: "/home/dwr/.lesshst"},
				{Path: "/home/dwr/.myapp"},
				{Path: "/home/dwr/.wgetrc"},
			}, map[xdg.Target]plan.TargetState{
				"/home/dwr/.config/wget/wgetrc": plan.TargetOccupied{},
			})

			// When rendering the plan
			report := Plan(actions, "/home/dwr", baseDirs)

			// Then every case is listed under a heading of its own with
			// unmanaged apps trailing them with counts at the bottom
			Expect(report).To(BeComparableTo(planHeader(false) +
				`bash
  skip    .bash_profile: unsupported
          read from a fixed path before any variable applies
  skip    .bashrc: unsupported
          read from a fixed path before any variable applies

git
  move    .gitconfig -> /home/dwr/.config/git/config

less
  symlink .lesshst -> /home/dwr/.local/state/less/history
  note    set LESSHISTFILE to make less use the new path directly

wget
  skip    .wgetrc: occupied
          /home/dwr/.config/wget/wgetrc already exists

unmanaged
  .foorc
  .myapp

2 to relocate, 5 skipped (1 occupied). Nothing has changed.
Run: xdg-housekeeper apply
`))
		})
	})
})

var _ = Describe("Applying", func() {
	It("should list the plan without counting anything", func() {
		// Given a plan holding two relocations and one skip
		actions := actionsFor([]plan.Entry{
			{Path: "/home/dwr/.gitconfig"},
			{Path: "/home/dwr/.lesshst"},
			{Path: "/home/dwr/.wgetrc"},
		}, map[xdg.Target]plan.TargetState{
			"/home/dwr/.config/wget/wgetrc": plan.TargetOccupied{},
		})

		// When reporting an apply before anything is carried out
		report := Applying(actions, "/home/dwr", baseDirs)

		// Then every action line reads the same as the plan, only the opening
		// differs, and no closing count follows
		Expect(report).To(BeComparableTo(planHeader(true) +
			`git
  move    .gitconfig -> /home/dwr/.config/git/config

less
  symlink .lesshst -> /home/dwr/.local/state/less/history
  note    set LESSHISTFILE to make less use the new path directly

wget
  skip    .wgetrc: occupied
          /home/dwr/.config/wget/wgetrc already exists
`))
	})
})

var _ = Describe("Outcomes", func() {
	Context("when no action failed", func() {
		It("should close with the count alone", func() {
			// Given a plan whose listing the user has already read
			actions := actionsFor([]plan.Entry{
				{Path: "/home/dwr/.gitconfig"},
				{Path: "/home/dwr/.lesshst"},
			}, nil)

			// When reporting outcomes in which nothing failed
			report := Outcomes(actions, nil)

			// Then only the count follows what the user already read
			Expect(report).To(BeComparableTo(
				`
2 relocated, 0 skipped.
`))
		})
	})

	Context("when an action failed", func() {
		It("should name only the apps whose actions failed", func() {
			// Given a plan whose listing the user has already read
			// and a failed relocation action
			actions := actionsFor([]plan.Entry{
				{Path: "/home/dwr/.gitconfig"},
				{Path: "/home/dwr/.lesshst"},
			}, nil)
			failed := map[plan.Relocation]error{
				relocationOf(actions, "/home/dwr/.gitconfig"): deniedMkdir(),
			}

			// When reporting the outcomes
			report := Outcomes(actions, failed)

			// Then only the failed relocation is spelled out
			Expect(report).To(BeComparableTo(
				`
git
  failed  create target parent: mkdir /home/dwr/.config/git: permission denied

1 relocated, 0 skipped, 1 failed.
`))
		})
	})
})

var _ = Describe("Declined", func() {
	It("should close with the plan's count alone", func() {
		// Given a plan whose listing the user has already read
		actions := actionsFor([]plan.Entry{
			{Path: "/home/dwr/.gitconfig"},
			{Path: "/home/dwr/.lesshst"},
			{Path: "/home/dwr/.wgetrc"},
		}, map[xdg.Target]plan.TargetState{
			"/home/dwr/.config/wget/wgetrc": plan.TargetOccupied{},
		})

		// When reporting an apply whose confirmation was declined
		report := Declined(actions)

		// Then the count says nothing has changed
		Expect(report).To(BeComparableTo(
			`
2 to relocate, 1 skipped (1 occupied). Nothing has changed.
`))
	})
})

// planHeader renders the header for both the plan and apply commands, printing
// the home and base directories.
func planHeader(isApply bool) string {
	baseDirsBlock := `base directories
  config  /home/dwr/.config
  data    /home/dwr/.local/share
  state   /home/dwr/.local/state
  cache   /home/dwr/.cache

`

	if isApply {
		return "xdg-housekeeper: applying to /home/dwr\n\n" + baseDirsBlock
	} else {
		return "xdg-housekeeper: plan for /home/dwr\n\n" + baseDirsBlock
	}
}

// deniedMkdir returns the error when a relocation fails due to a blocked path.
func deniedMkdir() error {
	return errors.New("create target parent: " +
		"mkdir /home/dwr/.config/git: permission denied")
}

// actionsFor returns the corresponding plan for the specified entries, with
// states standing in for what a probe found at their targets.
func actionsFor(
	entries []plan.Entry,
	states map[xdg.Target]plan.TargetState,
) plan.Plan {
	GinkgoHelper()

	survey, err := plan.Claim(entries, ruleTable(), baseDirs)
	Expect(err).NotTo(HaveOccurred())

	return plan.Build(survey, states)
}

// ruleTable returns the rule table the specs plan against: a move rule for
// git, symlink rules for less and wget, and the two entries bash keeps in
// place.
func ruleTable() rules.Table {
	GinkgoHelper()

	table, err := rules.BuildTable(
		[]rules.Rule{
			{
				App:        "git",
				EntryName:  ".gitconfig",
				Location:   xdg.LocationConfig,
				RelPath:    "git/config",
				Relocation: rules.RelocationMove{},
			},
			{
				App:       "less",
				EntryName: ".lesshst",
				Location:  xdg.LocationState,
				RelPath:   "less/history",
				Relocation: rules.RelocationSymlink{
					Note: "set LESSHISTFILE to make less use the new path " +
						"directly",
				},
			},
			{
				App:       "wget",
				EntryName: ".wgetrc",
				Location:  xdg.LocationConfig,
				RelPath:   "wget/wgetrc",
				Relocation: rules.RelocationSymlink{
					Note: "set WGETRC to make wget use the new path directly",
				},
			},
		},
		[]rules.UnsupportedEntry{
			{
				App:       "bash",
				EntryName: ".bashrc",
				Obstacle:  "read from a fixed path before any variable applies",
			},
			{
				App:       "bash",
				EntryName: ".bash_profile",
				Obstacle:  "read from a fixed path before any variable applies",
			},
		},
	)
	Expect(err).NotTo(HaveOccurred())

	return table
}

// relocationOf returns the relocation the plan holds for the entry at
// entryPath, failing the spec if the plan relocates no such entry.
func relocationOf(actions plan.Plan, entryPath string) plan.Relocation {
	GinkgoHelper()

	action, found := lo.Find(actions, func(action plan.Action) bool {
		return action.EntryPath() == entryPath
	})
	Expect(found).To(BeTrue())

	relocation, relocates := action.(plan.Relocation)
	Expect(relocates).To(BeTrue())

	return relocation
}
