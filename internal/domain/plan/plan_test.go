package plan

import (
	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/rules"
	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/xdg"
	"github.com/google/go-cmp/cmp"
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

var _ = Describe("Claim", func() {
	Context("when a rule claims an entry in home", func() {
		It("should report the entry and the target it resolves to", func() {
			// Given an entry in home that a rule claims
			entries := []Entry{{Path: "/home/dwr/.gitconfig"}}

			// When claiming the entries
			survey, err := Claim(entries, baseDirs)

			// Then the rule that claims it reports the entry and its target
			Expect(err).NotTo(HaveOccurred())
			Expect(survey).To(BeComparableTo(Survey{
				entries: entries,
				candidates: []Candidate{
					{
						Rule:   gitRule(),
						Entry:  Entry{Path: "/home/dwr/.gitconfig"},
						Target: "/home/dwr/.config/git/config",
					},
				},
			}, cmp.AllowUnexported(Survey{})))
		})
	})

	Context("when no rule claims an entry in home", func() {
		It("should produce no candidate for it", func() {
			// Given an entry in home that no rule claims
			entries := []Entry{{Path: "/home/dwr/.unclaimed"}}

			// When claiming the entries
			survey, err := Claim(entries, baseDirs)

			// Then there is nothing to relocate
			Expect(err).NotTo(HaveOccurred())
			Expect(survey).To(BeComparableTo(Survey{
				entries:    entries,
				candidates: []Candidate{},
			}, cmp.AllowUnexported(Survey{})))
		})
	})
})

var _ = Describe("Targets", func() {
	It("should report the target of every candidate", func() {
		// Given a survey holding a candidate for each of two entries
		entries := []Entry{
			{Path: "/home/dwr/.gitconfig"},
			{Path: "/home/dwr/.lesshst"},
		}
		survey := Survey{
			entries: entries,
			candidates: []Candidate{
				{
					Rule:   gitRule(),
					Entry:  entries[0],
					Target: "/home/dwr/.config/git/config",
				},
				{
					Rule:   lessRule(),
					Entry:  entries[1],
					Target: "/home/dwr/.local/state/less/history",
				},
			},
		}

		// When asking which targets have to be probed
		targets := survey.Targets()

		// Then every candidate's target is reported, in the order they sit
		Expect(targets).To(BeComparableTo([]xdg.Target{
			"/home/dwr/.config/git/config",
			"/home/dwr/.local/state/less/history",
		}))
	})
})

var _ = Describe("Build", func() {
	Context("when a move rule claims an entry and its target is free", func() {
		It("should relocate the entry to its target", func() {
			// Given an entry a move rule claims, and nothing at its target
			entries := []Entry{{Path: "/home/dwr/.gitconfig"}}
			survey := Survey{
				entries: entries,
				candidates: []Candidate{
					{
						Rule:   gitRule(),
						Entry:  entries[0],
						Target: "/home/dwr/.config/git/config",
					},
				},
			}

			// When building the plan
			actions := Build(survey, map[xdg.Target]TargetState{})

			// Then the entry is moved to its target
			Expect(actions).To(BeComparableTo(Plan{
				Move{
					app:       "git",
					entryPath: "/home/dwr/.gitconfig",
					target:    "/home/dwr/.config/git/config",
				},
			}, actionFields()))
		})
	})

	Context("when a symlink rule claims an entry and its target is free", func() {
		It("should relocate the entry and leave a link at the old path", func() {
			// Given an entry a symlink rule claims, and nothing at its target
			entries := []Entry{{Path: "/home/dwr/.lesshst"}}
			survey := Survey{
				entries: entries,
				candidates: []Candidate{
					{
						Rule:   lessRule(),
						Entry:  entries[0],
						Target: "/home/dwr/.local/state/less/history",
					},
				},
			}

			// When building the plan
			actions := Build(survey, map[xdg.Target]TargetState{})

			// Then the entry is linked back from the path it left
			Expect(actions).To(BeComparableTo(Plan{
				Symlink{
					app:       "less",
					entryPath: "/home/dwr/.lesshst",
					target:    "/home/dwr/.local/state/less/history",
					note:      "set LESSHISTFILE to make less use the new path directly",
				},
			}, actionFields()))
		})
	})

	Context("when something already exists at an entry's target", func() {
		It("should skip the entry and name what is in the way", func() {
			// Given an entry a rule claims, whose target already exists
			entries := []Entry{{Path: "/home/dwr/.wgetrc"}}
			survey := Survey{
				entries: entries,
				candidates: []Candidate{
					{
						Rule:   wgetRule(),
						Entry:  entries[0],
						Target: "/home/dwr/.config/wget/wgetrc",
					},
				},
			}
			states := map[xdg.Target]TargetState{
				"/home/dwr/.config/wget/wgetrc": TargetOccupied{},
			}

			// When building the plan
			actions := Build(survey, states)

			// Then the entry stays where it is, and its target is named
			Expect(actions).To(BeComparableTo(Plan{
				Occupied{
					app:       "wget",
					entryPath: "/home/dwr/.wgetrc",
					target:    "/home/dwr/.config/wget/wgetrc",
				},
			}, actionFields()))
		})
	})

	Context("when a regular file exists in an entry's target path", func() {
		It("should skip the entry and name the file in the way", func() {
			// Given an entry a rule claims, whose target path is blocked by a file
			entries := []Entry{{Path: "/home/dwr/.wgetrc"}}
			survey := Survey{
				entries: entries,
				candidates: []Candidate{
					{
						Rule:   wgetRule(),
						Entry:  entries[0],
						Target: "/home/dwr/.config/wget/wgetrc",
					},
				},
			}
			states := map[xdg.Target]TargetState{
				"/home/dwr/.config/wget/wgetrc": TargetBlocked{
					Blocker: "/home/dwr/.config/wget",
				},
			}

			// When building the plan
			actions := Build(survey, states)

			// Then the entry stays where it is, naming the file in its way
			Expect(actions).To(BeComparableTo(Plan{
				Blocked{
					app:       "wget",
					entryPath: "/home/dwr/.wgetrc",
					blocker:   "/home/dwr/.config/wget",
				},
			}, actionFields()))
		})
	})

	Context("when an entry belongs to an app that cannot be relocated", func() {
		It("should skip the entry and say what stands in the way", func() {
			// Given an entry of an app that cannot be relocated at all
			entries := []Entry{{Path: "/home/dwr/.bashrc"}}

			// When building the plan
			actions := Build(Survey{entries: entries}, map[xdg.Target]TargetState{})

			// Then the entry stays where it is, stating the obstacle
			Expect(actions).To(BeComparableTo(Plan{
				Unsupported{
					app:       "bash",
					entryPath: "/home/dwr/.bashrc",
					obstacle:  "read from a fixed path before any variable applies",
				},
			}, actionFields()))
		})
	})

	Context("when no rule claims an entry", func() {
		It("should list the entry as one nothing manages", func() {
			// Given an entry in home that no rule claims
			entries := []Entry{{Path: "/home/dwr/.foorc"}}

			// When building the plan
			actions := Build(Survey{entries: entries}, map[xdg.Target]TargetState{})

			// Then the entry is listed, with no app and nothing to do
			Expect(actions).To(BeComparableTo(Plan{
				Unmanaged{entryPath: "/home/dwr/.foorc"},
			}, actionFields()))
		})
	})

	Context("when several apps own entries in home", func() {
		It("should order the actions by app, listing the rest last", func() {
			// Given entries of several apps, in no particular order
			entries := []Entry{
				{Path: "/home/dwr/.wgetrc"},
				{Path: "/home/dwr/.foorc"},
				{Path: "/home/dwr/.gitconfig"},
				{Path: "/home/dwr/.bashrc"},
				{Path: "/home/dwr/.lesshst"},
			}
			survey := Survey{
				entries: entries,
				candidates: []Candidate{
					{
						Rule:   wgetRule(),
						Entry:  entries[0],
						Target: "/home/dwr/.config/wget/wgetrc",
					},
					{
						Rule:   gitRule(),
						Entry:  entries[2],
						Target: "/home/dwr/.config/git/config",
					},
					{
						Rule:   lessRule(),
						Entry:  entries[4],
						Target: "/home/dwr/.local/state/less/history",
					},
				},
			}
			states := map[xdg.Target]TargetState{
				"/home/dwr/.config/wget/wgetrc": TargetOccupied{},
				"/home/dwr/.local/state/less/history": TargetBlocked{
					Blocker: "/home/dwr/.local/state/less",
				},
			}

			// When building the plan
			actions := Build(survey, states)

			// Then the apps are listed alphabetically, with unmanaged ones at the end
			Expect(actions).To(BeComparableTo(Plan{
				Unsupported{
					app:       "bash",
					entryPath: "/home/dwr/.bashrc",
					obstacle:  "read from a fixed path before any variable applies",
				},
				Move{
					app:       "git",
					entryPath: "/home/dwr/.gitconfig",
					target:    "/home/dwr/.config/git/config",
				},
				Blocked{
					app:       "less",
					entryPath: "/home/dwr/.lesshst",
					blocker:   "/home/dwr/.local/state/less",
				},
				Occupied{
					app:       "wget",
					entryPath: "/home/dwr/.wgetrc",
					target:    "/home/dwr/.config/wget/wgetrc",
				},
				Unmanaged{entryPath: "/home/dwr/.foorc"},
			}, actionFields()))
		})
	})

	Context("when one app owns several entries in home", func() {
		It("should order that app's entries by name", func() {
			// Given two entries of one app, in no particular order
			entries := []Entry{
				{Path: "/home/dwr/.gitconfig"},
				{Path: "/home/dwr/.gitattributes"},
			}
			gitAttributes := gitRule()
			gitAttributes.EntryName = ".gitattributes"
			gitAttributes.RelPath = "git/attributes"

			survey := Survey{
				entries: entries,
				candidates: []Candidate{
					{
						Rule:   gitRule(),
						Entry:  entries[0],
						Target: "/home/dwr/.config/git/config",
					},
					{
						Rule:   gitAttributes,
						Entry:  entries[1],
						Target: "/home/dwr/.config/git/attributes",
					},
				},
			}

			// When building the plan
			actions := Build(survey, map[xdg.Target]TargetState{})

			// Then the entry whose name sorts first leads
			Expect(actions).To(BeComparableTo(Plan{
				Move{
					app:       "git",
					entryPath: "/home/dwr/.gitattributes",
					target:    "/home/dwr/.config/git/attributes",
				},
				Move{
					app:       "git",
					entryPath: "/home/dwr/.gitconfig",
					target:    "/home/dwr/.config/git/config",
				},
			}, actionFields()))
		})
	})

	Context("when several entries in home are unmanaged", func() {
		It("should order them by name, after every app", func() {
			// Given an owned entry and unmanaged entries in no particular order
			entries := []Entry{
				{Path: "/home/dwr/.zzzrc"},
				{Path: "/home/dwr/.gitconfig"},
				{Path: "/home/dwr/.aaarc"},
			}
			survey := Survey{
				entries: entries,
				candidates: []Candidate{
					{
						Rule:   gitRule(),
						Entry:  entries[1],
						Target: "/home/dwr/.config/git/config",
					},
				},
			}

			// When building the plan
			actions := Build(survey, map[xdg.Target]TargetState{})

			// Then they trail the app, reading in name order themselves
			Expect(actions).To(BeComparableTo(Plan{
				Move{
					app:       "git",
					entryPath: "/home/dwr/.gitconfig",
					target:    "/home/dwr/.config/git/config",
				},
				Unmanaged{entryPath: "/home/dwr/.aaarc"},
				Unmanaged{entryPath: "/home/dwr/.zzzrc"},
			}, actionFields()))
		})
	})
})

// actionFields lets cmp read the fields an action keeps unexported.
func actionFields() cmp.Option {
	return cmp.AllowUnexported(
		Move{}, Symlink{}, Occupied{}, Blocked{}, Unsupported{}, Unmanaged{},
	)
}

// gitRule returns the rule claiming .gitconfig, which relocates by moving.
func gitRule() rules.Rule {
	return rules.Rule{
		App:        "git",
		EntryName:  ".gitconfig",
		Location:   xdg.LocationConfig,
		RelPath:    "git/config",
		Relocation: rules.RelocationMove{},
	}
}

// lessRule returns the rule claiming .lesshst, which relocates by symlink.
func lessRule() rules.Rule {
	return rules.Rule{
		App:       "less",
		EntryName: ".lesshst",
		Location:  xdg.LocationState,
		RelPath:   "less/history",
		Relocation: rules.RelocationSymlink{
			Note: "set LESSHISTFILE to make less use the new path directly",
		},
	}
}

// wgetRule returns the rule claiming .wgetrc, which relocates by symlink.
func wgetRule() rules.Rule {
	return rules.Rule{
		App:       "wget",
		EntryName: ".wgetrc",
		Location:  xdg.LocationConfig,
		RelPath:   "wget/wgetrc",
		Relocation: rules.RelocationSymlink{
			Note: "set WGETRC to make wget use the new path directly",
		},
	}
}
