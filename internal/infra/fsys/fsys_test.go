package fsys

import (
	"os"
	"path/filepath"

	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/plan"
	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/xdg"
	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
)

var _ = Describe("Traverse", func() {
	var homePath string

	BeforeEach(func() {
		// Given a home directory
		homePath = GinkgoT().TempDir()
	})

	Context("when home also contains plain files and nested ones", func() {
		It("should report only the dot-prefixed files and directories located directly in home", func() {
			// Given two entries, a file nested inside one, and a plain file
			writeFile(filepath.Join(homePath, ".gitconfig"))
			makeDir(filepath.Join(homePath, ".myapp"))
			writeFile(filepath.Join(homePath, ".myapp", "settings.json"))
			writeFile(filepath.Join(homePath, "Documents.txt"))

			// And base directories none of them sit at
			baseDirs := resolvedBaseDirs(xdg.Environment{Home: homePath})

			// When traversing home
			entries, err := Traverse(homePath, baseDirs)

			// Then neither the plain file nor the nested one is an entry
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(BeComparableTo([]plan.Entry{
				{Path: filepath.Join(homePath, ".gitconfig")},
				{Path: filepath.Join(homePath, ".myapp")},
			}))
		})
	})

	Context("when home contains the XDG default base directories", func() {
		It("should report none of them as entries", func() {
			// Given every default base directory in home, beside one entry
			makeDir(filepath.Join(homePath, ".config"))
			makeDir(filepath.Join(homePath, ".local"))
			makeDir(filepath.Join(homePath, ".cache"))
			writeFile(filepath.Join(homePath, ".gitconfig"))

			// And no XDG variable moving any of them
			baseDirs := resolvedBaseDirs(xdg.Environment{Home: homePath})

			// When traversing home
			entries, err := Traverse(homePath, baseDirs)

			// Then only the entry left over is returned
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(BeComparableTo([]plan.Entry{
				{Path: filepath.Join(homePath, ".gitconfig")},
			}))
		})
	})

	Context("when an XDG variable names a base directory somewhere else in home", func() {
		It("should report the directories the variable left behind", func() {
			// Given a base directory under a name of its own, beside an
			// ordinary directory sitting at the default name
			makeDir(filepath.Join(homePath, ".myconfig"))
			makeDir(filepath.Join(homePath, ".config"))
			writeFile(filepath.Join(homePath, ".gitconfig"))

			// And the variable naming that directory
			baseDirs := resolvedBaseDirs(xdg.Environment{
				Home:       homePath,
				ConfigHome: filepath.Join(homePath, ".myconfig"),
			})

			// When traversing home
			entries, err := Traverse(homePath, baseDirs)

			// Then only the directories not named by the base directories are returned
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(BeComparableTo([]plan.Entry{
				{Path: filepath.Join(homePath, ".config")},
				{Path: filepath.Join(homePath, ".gitconfig")},
			}))
		})
	})

	Context("when home cannot be read", func() {
		It("should report the failure rather than an empty home", func() {
			// Given a path in place of home that does not exist
			missingHomePath := filepath.Join(homePath, "nowhere")
			baseDirs := resolvedBaseDirs(xdg.Environment{Home: homePath})

			// When traversing home
			entries, err := Traverse(missingHomePath, baseDirs)

			// Then an appropriate error is reported
			Expect(err).To(HaveOccurred())
			Expect(entries).To(BeNil())
		})
	})
})

var _ = Describe("Probe", func() {
	var basePath string

	BeforeEach(func() {
		// Given a base directory
		basePath = GinkgoT().TempDir()
	})

	Context("when only some of the targets are occupied", func() {
		It("should report the ones something already exists at", func() {
			// Given a target something exists at, and one that is free
			occupiedTarget := filepath.Join(basePath, "git", "config")
			freeTarget := filepath.Join(basePath, "less", "history")

			makeDir(filepath.Dir(occupiedTarget))
			writeFile(occupiedTarget)

			// When probing the targets
			states, err := Probe(targets(occupiedTarget, freeTarget))

			// Then the free target is left out
			Expect(err).NotTo(HaveOccurred())
			Expect(states).To(BeComparableTo(map[xdg.Target]plan.TargetState{
				xdg.Target(occupiedTarget): plan.TargetOccupied{},
			}))
		})
	})

	Context("when a symlink pointing at nothing exists at the target", func() {
		It("should report the target as taken all the same", func() {
			// Given a symlink at the target pointing at a path that is missing
			danglingTarget := filepath.Join(basePath, "config")
			missingPath := filepath.Join(basePath, "nowhere")

			Expect(os.Symlink(missingPath, danglingTarget)).To(Succeed())

			// When probing the targets
			states, err := Probe(targets(danglingTarget))

			// Then the entry has nowhere to relocate to
			Expect(err).NotTo(HaveOccurred())
			Expect(states).To(BeComparableTo(map[xdg.Target]plan.TargetState{
				xdg.Target(danglingTarget): plan.TargetOccupied{},
			}))
		})
	})

	Context("when a regular file sits several levels above the target", func() {
		It("should report that file rather than the missing directory under it", func() {
			// Given a file instead of a directory on the way to the target
			blockedTarget := filepath.Join(basePath, "git", "nested", "config")
			blockingPath := filepath.Join(basePath, "git")

			writeFile(blockingPath)

			// When probing the targets
			states, err := Probe(targets(blockedTarget))

			// Then the blocker is the file, not the missing directory under it
			Expect(err).NotTo(HaveOccurred())
			Expect(states).To(BeComparableTo(map[xdg.Target]plan.TargetState{
				xdg.Target(blockedTarget): plan.TargetBlocked{
					Blocker: blockingPath,
				},
			}))
		})
	})

	Context("when a symlink to a directory sits above the target", func() {
		It("should leave the target free", func() {
			// Given a symlink where the target's parent would be, pointing at a
			// directory that also exists
			linkedTarget := filepath.Join(basePath, "git", "config")
			linkPath := filepath.Dir(linkedTarget)
			realPath := filepath.Join(basePath, "git-real")

			makeDir(realPath)
			Expect(os.Symlink(realPath, linkPath)).To(Succeed())

			// When probing the targets
			states, err := Probe(targets(linkedTarget))

			// Then nothing stands in the way of relocating to the target
			Expect(err).NotTo(HaveOccurred())
			Expect(states).To(BeComparableTo(map[xdg.Target]plan.TargetState{}))
		})
	})

	Context("when a symlink pointing at nothing sits above the target", func() {
		It("should report the link as standing in the way", func() {
			// Given a dangling symlink where the target's parent would be
			blockedTarget := filepath.Join(basePath, "git", "config")
			linkPath := filepath.Dir(blockedTarget)
			missingPath := filepath.Join(basePath, "nowhere")

			Expect(os.Symlink(missingPath, linkPath)).To(Succeed())

			// When probing the targets
			states, err := Probe(targets(blockedTarget))

			// Then the blocker is the dangling symlink
			Expect(err).NotTo(HaveOccurred())
			Expect(states).To(BeComparableTo(map[xdg.Target]plan.TargetState{
				xdg.Target(blockedTarget): plan.TargetBlocked{
					Blocker: linkPath,
				},
			}))
		})
	})
})

var _ = Describe("Symlink", func() {
	var homePath string

	BeforeEach(func() {
		// Given a home directory
		homePath = GinkgoT().TempDir()
	})

	It("should relocate the entry and point a link at where it went", func() {
		// Given an entry in home, and a target nested below missing parents
		entryPath := filepath.Join(homePath, ".lesshst")
		targetPath := filepath.Join(
			homePath, ".local", "state", "less", "history")

		writeFile(entryPath)

		// When relocating the entry and linking back to it
		err := Symlink(entryPath, xdg.Target(targetPath))

		// Then the link left in home points to the target it was relocated to
		Expect(err).NotTo(HaveOccurred())
		Expect(targetPath).To(BeAnExistingFile())
		Expect(os.Readlink(entryPath)).To(BeComparableTo(targetPath))
	})
})

var _ = Describe("Move", func() {
	var (
		entryPath  string
		targetPath string
	)

	BeforeEach(func() {
		// Given an entry in home, and a target nested below missing parents
		homePath := GinkgoT().TempDir()

		entryPath = filepath.Join(homePath, ".gitconfig")
		targetPath = filepath.Join(homePath, ".config", "git", "config")

		writeFile(entryPath)
	})

	It("should leave the entry at its target, with nothing behind", func() {
		// When relocating the entry to its target
		err := Move(entryPath, xdg.Target(targetPath))

		// Then the entry sits at its target, and its old path is gone
		Expect(err).NotTo(HaveOccurred())
		Expect(targetPath).To(BeAnExistingFile())
		Expect(entryPath).NotTo(BeAnExistingFile())
	})

	It("should create the target's parent directories as the specification asks", func() {
		// When relocating the entry to its target
		err := Move(entryPath, xdg.Target(targetPath))

		// Then each directory created holds the mode the specification asks
		Expect(err).NotTo(HaveOccurred())

		info, statErr := os.Stat(filepath.Dir(targetPath))
		Expect(statErr).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(BeComparableTo(os.FileMode(0o700)))
	})
})

// resolvedBaseDirs resolves the base directories the environment names.
func resolvedBaseDirs(environment xdg.Environment) xdg.BaseDirs {
	GinkgoHelper()

	baseDirs, err := xdg.Resolve(environment)
	Expect(err).NotTo(HaveOccurred())

	return baseDirs
}

// targets converts the paths a spec builds into the targets Probe takes.
func targets(paths ...string) []xdg.Target {
	return lo.Map(paths, func(path string, _ int) xdg.Target {
		return xdg.Target(path)
	})
}

func writeFile(path string) {
	GinkgoHelper()

	Expect(os.WriteFile(path, nil, 0o600)).To(Succeed())
}

func makeDir(path string) {
	GinkgoHelper()

	Expect(os.Mkdir(path, 0o700)).To(Succeed())
}
