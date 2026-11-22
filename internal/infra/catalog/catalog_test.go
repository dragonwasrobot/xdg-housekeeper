package catalog

import (
	"io/fs"
	"strings"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2/dsl/core"
	"github.com/onsi/ginkgo/v2/dsl/table"
	. "github.com/onsi/gomega"

	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/rules"
	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/xdg"
)

var _ = Describe("Load", func() {
	Context("when an app file declares a move rule", func() {
		It("should read the rule into the table", func() {
			// Given a directory holding one app file with a move rule
			dir := fstest.MapFS{
				"git.json": &fstest.MapFile{Data: []byte(gitAppFile)},
			}

			// When loading the rule table
			loaded, err := Load(dir)

			// Then the table holds the rule the file declares
			Expect(err).NotTo(HaveOccurred())

			want, err := rules.BuildTable([]rules.Rule{gitRule()}, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded).To(BeComparableTo(want,
				cmp.AllowUnexported(rules.Table{})))
		})
	})

	Context("when an app file declares a symlink rule", func() {
		It("should read the rule and its note into the table", func() {
			// Given a directory holding one app file with a symlink rule
			dir := fstest.MapFS{
				"less.json": &fstest.MapFile{Data: []byte(lessAppFile)},
			}

			// When loading the rule table
			loaded, err := Load(dir)

			// Then the table holds the rule, linked back with its note
			Expect(err).NotTo(HaveOccurred())

			want, err := rules.BuildTable([]rules.Rule{lessRule()}, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded).To(BeComparableTo(want,
				cmp.AllowUnexported(rules.Table{})))
		})
	})

	Context("when an app file declares an unsupported entry", func() {
		It("should read the entry and its obstacle into the table", func() {
			// Given a directory holding one app file with an unsupported entry
			dir := fstest.MapFS{
				"bash.json": &fstest.MapFile{Data: []byte(bashAppFile)},
			}

			// When loading the rule table
			loaded, err := Load(dir)

			// Then the table holds the entry, with what keeps it in place
			Expect(err).NotTo(HaveOccurred())

			want, err := rules.BuildTable(nil,
				[]rules.UnsupportedEntry{bashrcEntry()})
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded).To(BeComparableTo(want,
				cmp.AllowUnexported(rules.Table{})))
		})
	})

	Context("when several app files declare rules and unsupported entries", func() {
		It("should merge every declaration into one table", func() {
			// Given a directory holding app files for three apps
			dir := fstest.MapFS{
				"bash.json": &fstest.MapFile{Data: []byte(bashAppFile)},
				"git.json":  &fstest.MapFile{Data: []byte(gitAppFile)},
				"less.json": &fstest.MapFile{Data: []byte(lessAppFile)},
			}

			// When loading the rule table
			loaded, err := Load(dir)

			// Then the table holds what every file declares
			Expect(err).NotTo(HaveOccurred())

			want, err := rules.BuildTable(
				[]rules.Rule{gitRule(), lessRule()},
				[]rules.UnsupportedEntry{bashrcEntry()},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded).To(BeComparableTo(want,
				cmp.AllowUnexported(rules.Table{})))
		})
	})

	Context("when an app file cannot be read", func() {
		It("should refuse to load the table without calling it malformed",
			func() {
				// Given a directory where the app file for git should be
				dir := fstest.MapFS{
					"git.json": &fstest.MapFile{Mode: fs.ModeDir},
				}

				// When loading the rule table
				_, err := Load(dir)

				// Then the table is refused for the read, not the content
				Expect(err).To(HaveOccurred())
				Expect(err).NotTo(MatchError(ErrMalformedAppFile))
			})
	})

	Context("when an app file is malformed", func() {
		table.DescribeTable("should refuse to load the table",
			func(from string, to string, want error) {
				// Given a directory holding the git app file, one value altered
				file := strings.Replace(gitAppFile, from, to, 1)
				dir := fstest.MapFS{
					"git.json": &fstest.MapFile{Data: []byte(file)},
				}

				// When loading the rule table
				_, err := Load(dir)

				// Then the table is refused, naming what is wrong
				Expect(err).To(MatchError(want))
			},
			table.Entry("because it names a relocation the tool does not know",
				`"kind": "move"`, `"kind": "copy"`,
				ErrUnknownRelocation),
			table.Entry("because it names a location the XDG specification does not define",
				`"location": "config"`, `"location": "configuration"`,
				ErrUnknownLocation),
			table.Entry("because it gives a move a note",
				`{"kind": "move"}`,
				`{"kind": "move", "note": "set GIT_CONFIG_GLOBAL"}`,
				ErrNoteOnMove),
			table.Entry("because it carries a key the format does not define",
				`"relPath": "git/config"`, `"rel_path": "git/config"`,
				ErrMalformedAppFile),
			table.Entry("because it holds more than one JSON document",
				"\n}", "\n}\n{}",
				ErrMalformedAppFile),
			table.Entry("because it names no app",
				`"app": "git",`, ``,
				ErrMissingApp),
		)
	})
})

// gitAppFile is the app file for git, declaring a move rule for .gitconfig.
const gitAppFile = `{
  "app": "git",
  "rules": [
    {
      "entryName": ".gitconfig",
      "location": "config",
      "relPath": "git/config",
      "relocation": {"kind": "move"}
    }
  ]
}`

// lessAppFile is the app file for less, declaring a symlink rule for .lesshst.
const lessAppFile = `{
  "app": "less",
  "rules": [
    {
      "entryName": ".lesshst",
      "location": "state",
      "relPath": "less/history",
      "relocation": {
        "kind": "symlink",
        "note": "set LESSHISTFILE to make less use the new path directly"
      }
    }
  ]
}`

// bashAppFile is the app file for bash, declaring .bashrc an unsupported entry.
const bashAppFile = `{
  "app": "bash",
  "unsupported": [
    {
      "entryName": ".bashrc",
      "obstacle": "read from a fixed path before any variable applies"
    }
  ]
}`

// gitRule returns the rule gitAppFile declares.
func gitRule() rules.Rule {
	return rules.Rule{
		App:        "git",
		EntryName:  ".gitconfig",
		Location:   xdg.LocationConfig,
		RelPath:    "git/config",
		Relocation: rules.RelocationMove{},
	}
}

// lessRule returns the rule lessAppFile declares.
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

// bashrcEntry returns the unsupported entry bashAppFile declares.
func bashrcEntry() rules.UnsupportedEntry {
	return rules.UnsupportedEntry{
		App:       "bash",
		EntryName: ".bashrc",
		Obstacle:  "read from a fixed path before any variable applies",
	}
}
