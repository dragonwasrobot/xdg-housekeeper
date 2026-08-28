package xdg

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestResolve_ShouldFallBackToTheSpecifiedDefaultsWhenNothingIsSet(t *testing.T) {
	// Given a home directory and no XDG variables are set
	environment := Environment{Home: "/home/dwr"}

	// When resolving the base directories
	got, err := Resolve(environment)
	if err != nil {
		t.Fatalf("resolve base directories: %v", err)
	}

	// Then every location falls back to its default under home
	want := BaseDirs{
		config: "/home/dwr/.config",
		data:   "/home/dwr/.local/share",
		state:  "/home/dwr/.local/state",
		cache:  "/home/dwr/.cache",
	}

	if diff := cmp.Diff(want, got, cmp.AllowUnexported(BaseDirs{})); diff != "" {
		t.Errorf("base directories mismatch (-want +got):\n%s", diff)
	}
}

func TestResolve_ShouldChooseAnAbsolutePathOverTheDefault(t *testing.T) {
	// Given every XDG variable is set to an absolute path outside home
	environment := Environment{
		Home:       "/home/dwr",
		ConfigHome: "/etc/dwr/config",
		DataHome:   "/mnt/data",
		StateHome:  "/var/dwr/state",
		CacheHome:  "/var/tmp/dwr",
	}

	// When resolving the base directories
	got, err := Resolve(environment)
	if err != nil {
		t.Fatalf("resolve base directories: %v", err)
	}

	// Then each location takes the path its variable specifies
	want := BaseDirs{
		config: "/etc/dwr/config",
		data:   "/mnt/data",
		state:  "/var/dwr/state",
		cache:  "/var/tmp/dwr",
	}

	if diff := cmp.Diff(want, got, cmp.AllowUnexported(BaseDirs{})); diff != "" {
		t.Errorf("base directories mismatch (-want +got):\n%s", diff)
	}
}

func TestResolve_ShouldIgnoreRedundantSeparators(t *testing.T) {
	// Given every XDG variable is set to an absolute path whose spelling
	// carries doubled or trailing separators
	environment := Environment{
		Home:       "/home/dwr",
		ConfigHome: "/etc//dwr/config",
		DataHome:   "/mnt/data/",
		StateHome:  "/var//dwr//state",
		CacheHome:  "/var/tmp/dwr//",
	}

	// When resolving the base directories
	got, err := Resolve(environment)
	if err != nil {
		t.Fatalf("resolve base directories: %v", err)
	}

	// Then each location takes the directory that variable specifies, with the
	// redundant separators gone
	want := BaseDirs{
		config: "/etc/dwr/config",
		data:   "/mnt/data",
		state:  "/var/dwr/state",
		cache:  "/var/tmp/dwr",
	}

	if diff := cmp.Diff(want, got, cmp.AllowUnexported(BaseDirs{})); diff != "" {
		t.Errorf("base directories mismatch (-want +got):\n%s", diff)
	}
}

func TestResolve_ShouldTreatARelativePathAsUnset(t *testing.T) {
	// Given every XDG variable is set to a relative path, which the
	// specification declares invalid
	environment := Environment{
		Home:       "/home/dwr",
		ConfigHome: "config",
		DataHome:   "./data",
		StateHome:  "../state",
		CacheHome:  "var/tmp/dwr",
	}

	// When resolving the base directories
	got, err := Resolve(environment)
	if err != nil {
		t.Fatalf("resolve base directories: %v", err)
	}

	// Then every location falls back to its default under home
	want := BaseDirs{
		config: "/home/dwr/.config",
		data:   "/home/dwr/.local/share",
		state:  "/home/dwr/.local/state",
		cache:  "/home/dwr/.cache",
	}

	if diff := cmp.Diff(want, got, cmp.AllowUnexported(BaseDirs{})); diff != "" {
		t.Errorf("base directories mismatch (-want +got):\n%s", diff)
	}
}

func TestResolve_ShouldTreatTheFilesystemRootAsUnset(t *testing.T) {
	// Given every XDG variable specifies the filesystem root, which a shell
	// expanding an unset variable can produce by accident
	environment := Environment{
		Home:       "/home/dwr",
		ConfigHome: "/",
		DataHome:   "/",
		StateHome:  "/",
		CacheHome:  "/",
	}

	// When resolving the base directories
	got, err := Resolve(environment)
	if err != nil {
		t.Fatalf("resolve base directories: %v", err)
	}

	// Then every location falls back to its default under home, rather than
	// offering to relocate an entry to the root of the filesystem
	want := BaseDirs{
		config: "/home/dwr/.config",
		data:   "/home/dwr/.local/share",
		state:  "/home/dwr/.local/state",
		cache:  "/home/dwr/.cache",
	}

	if diff := cmp.Diff(want, got, cmp.AllowUnexported(BaseDirs{})); diff != "" {
		t.Errorf("base directories mismatch (-want +got):\n%s", diff)
	}
}

func TestResolve_ShouldFailWhenTheHomeDirectoryIsNotAnAbsolutePath(t *testing.T) {
	tests := []struct {
		name string
		home string
	}{
		{name: "should fail when the home directory is not set", home: ""},
		{name: "should fail when the home directory is relative", home: "dwr"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a home directory variable that is not an absolute path
			environment := Environment{Home: tt.home}

			// When resolving the base directories
			_, err := Resolve(environment)

			// Then it refuses to resolve anything against it
			if !errors.Is(err, ErrInvalidHome) {
				t.Errorf("got error %v, want ErrInvalidHome", err)
			}
		})
	}
}

func TestTarget_ShouldJoinAPathOntoItsLocationsBaseDirectory(t *testing.T) {
	// Given base directories resolved from a home directory
	baseDirs, err := Resolve(Environment{Home: "/home/dwr"})
	if err != nil {
		t.Fatalf("resolve base directories: %v", err)
	}

	tests := []struct {
		name     string
		location Location
		relPath  string
		want     Target
	}{
		{
			name:     "should resolve a target under the config base",
			location: LocationConfig,
			relPath:  "git/config",
			want:     "/home/dwr/.config/git/config",
		},
		{
			name:     "should resolve a target under the data base",
			location: LocationData,
			relPath:  "gnupg",
			want:     "/home/dwr/.local/share/gnupg",
		},
		{
			name:     "should resolve a target under the state base",
			location: LocationState,
			relPath:  "less/history",
			want:     "/home/dwr/.local/state/less/history",
		},
		{
			name:     "should resolve a target under the cache base",
			location: LocationCache,
			relPath:  "wget",
			want:     "/home/dwr/.cache/wget",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When resolving a path relative to that location's base
			got, err := baseDirs.Target(tt.relPath, tt.location)
			if err != nil {
				t.Fatalf("resolve target: %v", err)
			}

			// Then the target exists inside the base directory
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("target mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTarget_ShouldRejectAPathThatDoesNotLiveInsideItsBaseDirectory(t *testing.T) {
	// Given base directories resolved from a home directory
	baseDirs, err := Resolve(Environment{Home: "/home/dwr"})
	if err != nil {
		t.Fatalf("resolve base directories: %v", err)
	}

	tests := []struct {
		name    string
		relPath string
	}{
		{
			name:    "should reject a path that climbs out of the base directory",
			relPath: "../../../etc/passwd",
		},
		{
			name:    "should reject an absolute path",
			relPath: "/etc/passwd",
		},
		{
			name:    "should reject a path that specifies the base directory itself",
			relPath: ".",
		},
		{
			name:    "should reject a path that specifies nothing at all",
			relPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When resolving a path that does not point to something
			// inside that base directory
			_, err := baseDirs.Target(tt.relPath, LocationConfig)

			// Then it refuses to name a target at all
			if !errors.Is(err, ErrInvalidTarget) {
				t.Errorf("got error %v, want ErrInvalidTarget", err)
			}
		})
	}
}

func TestTarget_ShouldRejectALocationTheSpecificationDoesNotDefine(t *testing.T) {
	// Given base directories resolved from a home directory
	baseDirs, err := Resolve(Environment{Home: "/home/dwr"})
	if err != nil {
		t.Fatalf("resolve base directories: %v", err)
	}

	// When resolving an otherwise valid path against a location outside the
	// four the specification defines
	_, err = baseDirs.Target("git/config", Location(""))

	// Then it refuses to name a target at all
	if !errors.Is(err, ErrInvalidTarget) {
		t.Errorf("got error %v, want ErrInvalidTarget", err)
	}
}
