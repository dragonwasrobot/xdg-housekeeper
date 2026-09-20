package xdg

import (
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2/dsl/core"
	"github.com/onsi/ginkgo/v2/dsl/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resolve", func() {
	table.DescribeTable("should fall back to the specified defaults",
		func(environment Environment) {
			// When resolving the base directories
			baseDirs, err := Resolve(environment)

			// Then every location falls back to its default under home
			Expect(err).NotTo(HaveOccurred())
			Expect(baseDirs).To(BeComparableTo(defaultsUnderHome(),
				baseDirFields()))
		},
		table.Entry("because no XDG variables are set",
			Environment{Home: "/home/dwr"}),
		// The specification declares a relative path invalid.
		table.Entry("because every variable specifies a relative path",
			Environment{
				Home:       "/home/dwr",
				ConfigHome: "config",
				DataHome:   "./data",
				StateHome:  "../state",
				CacheHome:  "var/tmp/dwr",
			}),
		// A shell expanding an unset variable can produce the root by
		// accident, and it is no place to relocate an entry to.
		table.Entry("because every variable specifies the filesystem root",
			Environment{
				Home:       "/home/dwr",
				ConfigHome: "/",
				DataHome:   "/",
				StateHome:  "/",
				CacheHome:  "/",
			}),
	)

	Context("when every XDG variable is set to an absolute path", func() {
		It("should take the path each variable specifies", func() {
			// Given every XDG variable is set to an absolute path outside home
			environment := Environment{
				Home:       "/home/dwr",
				ConfigHome: "/etc/dwr/config",
				DataHome:   "/mnt/data",
				StateHome:  "/var/dwr/state",
				CacheHome:  "/var/tmp/dwr",
			}

			// When resolving the base directories
			baseDirs, err := Resolve(environment)

			// Then each location takes the path its variable specifies
			Expect(err).NotTo(HaveOccurred())
			Expect(baseDirs).To(BeComparableTo(dirsOutsideHome(),
				baseDirFields()))
		})
	})

	Context("when a path carries doubled or trailing separators", func() {
		It("should take that directory with the separators gone", func() {
			// Given every XDG variable is set to an absolute path whose
			// spelling carries doubled or trailing separators
			environment := Environment{
				Home:       "/home/dwr",
				ConfigHome: "/etc//dwr/config",
				DataHome:   "/mnt/data/",
				StateHome:  "/var//dwr//state",
				CacheHome:  "/var/tmp/dwr//",
			}

			// When resolving the base directories
			baseDirs, err := Resolve(environment)

			// Then each location takes the directory that variable specifies,
			// with the redundant separators gone
			Expect(err).NotTo(HaveOccurred())
			Expect(baseDirs).To(BeComparableTo(dirsOutsideHome(),
				baseDirFields()))
		})
	})

	Context("when the home directory is not an absolute path", func() {
		table.DescribeTable("should refuse to resolve anything against it",
			func(home string) {
				// Given a home directory variable that is not an absolute path
				environment := Environment{Home: home}

				// When resolving the base directories
				_, err := Resolve(environment)

				// Then it refuses to resolve anything against it
				Expect(err).To(MatchError(ErrInvalidHome))
			},
			table.Entry("because nothing is set", ""),
			table.Entry("because the path is relative", "dwr"),
		)
	})
})

var _ = Describe("Target", func() {
	var baseDirs BaseDirs

	BeforeEach(func() {
		// Given base directories resolved from a home directory
		var err error

		baseDirs, err = Resolve(Environment{Home: "/home/dwr"})
		Expect(err).NotTo(HaveOccurred())
	})

	table.DescribeTable("should join a path onto its location's base directory",
		func(location Location, relPath string, want Target) {
			// When resolving a path relative to that location's base
			target, err := baseDirs.Target(relPath, location)

			// Then the target exists inside the base directory
			Expect(err).NotTo(HaveOccurred())
			Expect(target).To(BeComparableTo(want))
		},
		table.Entry("for the config location",
			LocationConfig, "git/config",
			Target("/home/dwr/.config/git/config")),
		table.Entry("for the data location",
			LocationData, "gnupg",
			Target("/home/dwr/.local/share/gnupg")),
		table.Entry("for the state location",
			LocationState, "less/history",
			Target("/home/dwr/.local/state/less/history")),
		table.Entry("for the cache location",
			LocationCache, "wget",
			Target("/home/dwr/.cache/wget")),
	)

	Context("when the path does not point inside its base directory", func() {
		table.DescribeTable("should refuse to name a target at all",
			func(relPath string) {
				// When resolving a path that does not point to something
				// inside that base directory
				_, err := baseDirs.Target(relPath, LocationConfig)

				// Then it refuses to name a target at all
				Expect(err).To(MatchError(ErrInvalidTarget))
			},
			table.Entry("because it climbs out of it", "../../../etc/passwd"),
			table.Entry("because it is absolute", "/etc/passwd"),
			table.Entry("because it specifies the base directory itself", "."),
			table.Entry("because it specifies nothing at all", ""),
		)
	})

	Context("when the specification does not define the location", func() {
		It("should abort rather than name a target", func() {
			// When resolving an otherwise valid path against a location
			// outside the four the specification defines
			resolve := func() {
				//nolint:errcheck // it panics before it can return an error
				baseDirs.Target("git/config", Location(""))
			}

			// Then the tool panics rather than reporting a failure
			Expect(resolve).To(Panic())
		})
	})
})

// defaultsUnderHome returns the XDG specification's default base directories.
func defaultsUnderHome() BaseDirs {
	return BaseDirs{
		config: "/home/dwr/.config",
		data:   "/home/dwr/.local/share",
		state:  "/home/dwr/.local/state",
		cache:  "/home/dwr/.cache",
	}
}

// dirsOutsideHome returns a set of base directories all pointing outside home.
func dirsOutsideHome() BaseDirs {
	return BaseDirs{
		config: "/etc/dwr/config",
		data:   "/mnt/data",
		state:  "/var/dwr/state",
		cache:  "/var/tmp/dwr",
	}
}

func baseDirFields() cmp.Option {
	return cmp.AllowUnexported(BaseDirs{})
}
