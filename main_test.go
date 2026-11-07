package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing/iotest"

	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"

	"github.com/dragonwasrobot/xdg-housekeeper/internal/infra/cli"
)

var _ = Describe("run", func() {
	var home string

	BeforeEach(func() {
		// Given a home directory and no XDG variables are set
		home = GinkgoT().TempDir()

		for _, name := range []string{
			"XDG_CONFIG_HOME", "XDG_DATA_HOME",
			"XDG_STATE_HOME", "XDG_CACHE_HOME",
		} {
			GinkgoT().Setenv(name, "")
		}
	})

	Context("when making a plan for a home directory with an entry a move rule claims", func() {
		It("should report the move and leave the entry where it is", func() {
			// Given a single entry in home for a known application
			entryPath := filepath.Join(home, ".gitconfig")
			Expect(os.WriteFile(entryPath, []byte("[user]\n"), 0o600)).
				To(Succeed())

			// When the run derives its plan
			var stdin, stdout, stderr bytes.Buffer

			code := run([]string{"--home", home, "plan"}, &stdin, &stdout, &stderr)

			// Then the entry is claimed and the target resolved
			Expect(code).To(BeComparableTo(exitSuccess))
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(ContainSubstring(
				"move    .gitconfig -> " +
					filepath.Join(home, ".config", "git", "config")))
			Expect(entryPath).To(BeAnExistingFile())
			Expect(filepath.Join(home, ".config")).
				NotTo(BeAnExistingFile())
		})
	})

	Context("when no home is specified", func() {
		It("should work against the home the environment names", func() {
			// Given the environment names a home with a claimed entry
			GinkgoT().Setenv("HOME", home)

			Expect(os.WriteFile(
				filepath.Join(home, ".gitconfig"), []byte("x\n"), 0o600,
			)).To(Succeed())

			// When the run derives its plan
			var stdin, stdout, stderr bytes.Buffer

			code := run([]string{"plan"}, &stdin, &stdout, &stderr)

			// Then the entry is bound for a target under that home
			Expect(code).To(BeComparableTo(exitSuccess))
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(ContainSubstring(
				"move    .gitconfig -> " +
					filepath.Join(home, ".config", "git", "config")))
		})
	})

	Context("when the XDG variables name directories of their own", func() {
		It("should resolve every base directory where its variable says", func() {
			// Given every variable naming a directory outside home, and an
			// entry a move rule claims
			xdgHome := GinkgoT().TempDir()

			GinkgoT().Setenv("XDG_CONFIG_HOME", filepath.Join(xdgHome, "config"))
			GinkgoT().Setenv("XDG_DATA_HOME", filepath.Join(xdgHome, "data"))
			GinkgoT().Setenv("XDG_STATE_HOME", filepath.Join(xdgHome, "state"))
			GinkgoT().Setenv("XDG_CACHE_HOME", filepath.Join(xdgHome, "cache"))

			Expect(os.WriteFile(
				filepath.Join(home, ".gitconfig"), []byte("x\n"), 0o600,
			)).To(Succeed())

			// When the run derives its plan
			var stdin, stdout, stderr bytes.Buffer

			code := run([]string{"--home", home, "plan"}, &stdin, &stdout, &stderr)

			// Then each location resolved to its corresponding variable's
			// value, and each entry is bound for one of them
			Expect(code).To(BeComparableTo(exitSuccess))
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(ContainSubstring(fmt.Sprintf(
				`base directories
  config  %[1]s/config
  data    %[1]s/data
  state   %[1]s/state
  cache   %[1]s/cache
`, xdgHome)))
			Expect(stdout.String()).To(ContainSubstring(
				"move    .gitconfig -> " +
					filepath.Join(xdgHome, "config", "git", "config")))
		})
	})

	Context("when applied in a home where one entry's target is occupied", func() {
		It("should relocate the rest and report what it did", func() {
			// Given entries a move rule and a symlink rule claim, and one
			// whose target is already occupied
			for _, name := range []string{".gitconfig", ".lesshst"} {
				Expect(os.WriteFile(
					filepath.Join(home, name), []byte("x\n"), 0o600,
				)).To(Succeed())
			}

			Expect(os.WriteFile(
				filepath.Join(home, ".wgetrc"), []byte("tries=3\n"), 0o600,
			)).To(Succeed())

			occupiedTarget := filepath.Join(home, ".config", "wget", "wgetrc")
			Expect(os.MkdirAll(filepath.Dir(occupiedTarget), 0o700)).
				To(Succeed())
			Expect(os.WriteFile(occupiedTarget, []byte("tries=9\n"), 0o600)).
				To(Succeed())

			// When the run is asked to carry out the plan
			var stdin, stdout, stderr bytes.Buffer

			code := run(
				[]string{"--home", home, "--yes", "apply"},
				&stdin, &stdout, &stderr)

			// Then the report closes on what came of the plan
			Expect(code).To(BeComparableTo(exitSuccess))
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).
				To(HaveSuffix("\n2 relocated, 1 skipped (1 occupied).\n"))

			// And the filesystem reflects it
			Expect(filepath.Join(home, ".gitconfig")).NotTo(BeAnExistingFile())
			Expect(filepath.Join(home, ".config", "git", "config")).
				To(BeAnExistingFile())

			linked := filepath.Join(home, ".local", "state", "less", "history")
			Expect(os.Readlink(filepath.Join(home, ".lesshst"))).
				To(BeComparableTo(linked))
			Expect(linked).To(BeAnExistingFile())

			Expect(os.ReadFile(filepath.Join(home, ".wgetrc"))).
				To(BeComparableTo([]byte("tries=3\n")))
			Expect(os.ReadFile(occupiedTarget)).
				To(BeComparableTo([]byte("tries=9\n")))
		})
	})

	Context("when applied in a home without being told to skip the prompt", func() {
		It("should carry the plan out once the answer confirms it", func() {
			// Given an entry a move rule claims
			Expect(os.WriteFile(
				filepath.Join(home, ".gitconfig"), []byte("x\n"), 0o600,
			)).To(Succeed())

			// When the run carries out the plan, and the answer confirms it
			stdin := bytes.NewBufferString("y\n")

			var stdout, stderr bytes.Buffer

			code := run(
				[]string{"--home", home, "apply"},
				stdin, &stdout, &stderr)

			// Then the listing is reported before the answer is asked for, and
			// the closing count only after it
			Expect(code).To(BeComparableTo(exitSuccess))
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(ContainSubstring(
				"move    .gitconfig -> " +
					filepath.Join(home, ".config", "git", "config") +
					"\n\nContinue? [y/N] \n1 relocated, 0 skipped.\n"))

			// And the entry has been relocated
			Expect(filepath.Join(home, ".gitconfig")).NotTo(BeAnExistingFile())
			Expect(filepath.Join(home, ".config", "git", "config")).
				To(BeAnExistingFile())
		})
	})

	Context("when applied in a home and the answer declines", func() {
		It("should leave every entry where it is, and still succeed", func() {
			// Given an entry a move rule claims
			entryPath := filepath.Join(home, ".gitconfig")
			Expect(os.WriteFile(entryPath, []byte("x\n"), 0o600)).
				To(Succeed())

			// When the run carries out the plan, and the answer declines it
			stdin := bytes.NewBufferString("n\n")

			var stdout, stderr bytes.Buffer

			code := run(
				[]string{"--home", home, "apply"},
				stdin, &stdout, &stderr)

			// Then the report closes on the plan's count, with nothing changed
			Expect(code).To(BeComparableTo(exitSuccess))
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(HaveSuffix(
				"\nContinue? [y/N] \n" +
					"1 to relocate, 0 skipped. Nothing has changed.\n"))

			// And nothing on disk moved
			Expect(entryPath).To(BeAnExistingFile())
			Expect(filepath.Join(home, ".config")).NotTo(BeAnExistingFile())
		})
	})

	Context("when applied in a home and no answer ever comes", func() {
		It("should leave every entry where it is, and still succeed", func() {
			// Given an entry a move rule claims
			entryPath := filepath.Join(home, ".gitconfig")
			Expect(os.WriteFile(entryPath, []byte("x\n"), 0o600)).
				To(Succeed())

			// When the run carries out the plan, and the stream the answer
			// comes from ends without giving one
			var stdin, stdout, stderr bytes.Buffer

			code := run(
				[]string{"--home", home, "apply"},
				&stdin, &stdout, &stderr)

			// Then the run declines, just as it does for an explicit no
			Expect(code).To(BeComparableTo(exitSuccess))
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(HaveSuffix(
				"\nContinue? [y/N] \n" +
					"1 to relocate, 0 skipped. Nothing has changed.\n"))

			// And nothing on disk moved
			Expect(entryPath).To(BeAnExistingFile())
		})
	})

	Context("when applied in a home and the answer cannot be read", func() {
		It("should report the failure, having moved nothing", func() {
			// Given an entry a move rule claims
			entryPath := filepath.Join(home, ".gitconfig")
			Expect(os.WriteFile(entryPath, []byte("x\n"), 0o600)).
				To(Succeed())

			// When the run carries out the plan, and the answer stream fails
			stdin := iotest.ErrReader(io.ErrClosedPipe)

			var stdout, stderr bytes.Buffer

			code := run(
				[]string{"--home", home, "apply"},
				stdin, &stdout, &stderr)

			// Then the exit code says the plan never ran, and failure reported
			Expect(code).To(BeComparableTo(exitNotStarted))
			Expect(stderr.String()).To(HavePrefix("xdg-housekeeper: "))

			// And nothing on disk moved
			Expect(entryPath).To(BeAnExistingFile())
		})
	})

	Context("when applied in a home whose base directory cannot be written", func() {
		It("should carry out the actions after the one that failed", func() {
			// Given two entries, with one whose target cannot be written to
			for _, name := range []string{".gitconfig", ".lesshst"} {
				Expect(os.WriteFile(
					filepath.Join(home, name), []byte("x\n"), 0o600,
				)).To(Succeed())
			}

			Expect(os.Mkdir(filepath.Join(home, ".config"), 0o500)).
				To(Succeed())

			// When the run carries out the plan
			var stdin, stdout, stderr bytes.Buffer

			code := run(
				[]string{"--home", home, "--yes", "apply"},
				&stdin, &stdout, &stderr)

			// Then only the closing count reaches stderr
			Expect(stderr.String()).
				To(BeComparableTo("xdg-housekeeper: 1 action failed\n"))
			Expect(code).To(BeComparableTo(exitFailure))

			// And the entry whose action failed stays in its original location
			Expect(filepath.Join(home, ".gitconfig")).To(BeAnExistingFile())
			Expect(os.Readlink(filepath.Join(home, ".lesshst"))).
				To(BeComparableTo(
					filepath.Join(home, ".local", "state", "less", "history")))
		})
	})

	Context("when the outcomes cannot be reported once the plan has run", func() {
		It("should close on the code that says some of the plan ran", func() {
			// Given an entry a move rule claims
			entryPath := filepath.Join(home, ".gitconfig")
			Expect(os.WriteFile(entryPath, []byte("x\n"), 0o600)).
				To(Succeed())

			// When the run carries out the plan, and the reporting stream
			// closes prematurely
			stdout := &closingWriter{writes: 1}

			var stdin, stderr bytes.Buffer

			code := run(
				[]string{"--home", home, "--yes", "apply"},
				&stdin, stdout, &stderr)

			// Then the code says the plan ran, since the entry has moved
			Expect(code).To(BeComparableTo(exitFailure))
			Expect(entryPath).NotTo(BeAnExistingFile())
		})
	})

	Context("when the home a run names is not an absolute path", func() {
		It("should report the failure on stderr alone, having done nothing", func() {
			// Given a run naming a home that nothing can be resolved against
			var stdin, stdout, stderr bytes.Buffer

			// When the run derives its plan
			code := run(
				[]string{"--home", "relative/path", "plan"},
				&stdin, &stdout, &stderr)

			// Then the exit code sets it apart from a run that got partway, and
			// the failure is named on stderr
			Expect(code).To(BeComparableTo(exitNotStarted))
			Expect(stdout.String()).To(BeEmpty())
			Expect(stderr.String()).To(HavePrefix("xdg-housekeeper: "))
		})
	})

	Context("when a command the tool does not define is named", func() {
		It("should report the failure and how the tool is used", func() {
			// Given a run naming no command the tool defines
			var stdin, stdout, stderr bytes.Buffer

			// When the run tries to execute it
			code := run([]string{"bogus"}, &stdin, &stdout, &stderr)

			// Then the usage description is presented after the failure
			Expect(code).To(BeComparableTo(exitNotStarted))
			Expect(stdout.String()).To(BeEmpty())
			Expect(stderr.String()).To(BeComparableTo(
				"xdg-housekeeper: unknown command \"bogus\"\n\n" + cli.Usage()))
		})
	})

	Context("when the run asks how the tool is used", func() {
		It("should report the usage on stdout, and succeed", func() {
			// Given a run naming the help command
			var stdin, stdout, stderr bytes.Buffer

			// When the run executes the command
			code := run([]string{"help"}, &stdin, &stdout, &stderr)

			// Then the usage description is presented
			Expect(code).To(BeComparableTo(exitSuccess))
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(BeComparableTo(cli.Usage()))
		})
	})
})

// closingWriter accepts the specified number of writes and refuses every one
// after them, simulating a stream closed partway through a run does.
type closingWriter struct {
	writes int
}

// Write implements io.Writer.
func (w *closingWriter) Write(p []byte) (int, error) {
	if w.writes == 0 {
		return 0, io.ErrClosedPipe
	}

	w.writes--

	return len(p), nil
}
