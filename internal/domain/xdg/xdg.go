// Package xdg holds the XDG Base Directory Specification's knowledge: which
// base directory each location has, and how one is resolved from the
// environment.
package xdg

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ErrInvalidHome reports that the home directory is unset or relative, leaving
// every location's default with nothing to resolve against.
var ErrInvalidHome = errors.New("home directory is not an absolute path")

// ErrInvalidTarget reports that a location and relative path name no target:
// the relative path is actually absolute, or it resolves outside its location's
// base directory.
var ErrInvalidTarget = errors.New("target is not inside an XDG base directory")

// Location specifies which XDG base directory an entry belongs in.
type Location string

const (
	// LocationConfig specifies the base directory holding configuration.
	LocationConfig Location = "config"
	// LocationData specifies the base directory holding portable data that
	// should be backed up.
	LocationData Location = "data"
	// LocationState specifies the base directory holding state that persists
	// but need not be kept.
	LocationState Location = "state"
	// LocationCache specifies the base directory holding non-essential,
	// regenerable data.
	LocationCache Location = "cache"
)

// Locations returns every location the XDG specification defines.
func Locations() []Location {
	return []Location{
		LocationConfig,
		LocationData,
		LocationState,
		LocationCache,
	}
}

// Environment holds the raw values a BaseDirs is resolved from: HOME and the
// four XDG variables. An empty field stands for a variable that is not set.
type Environment struct {
	Home       string // HOME
	ConfigHome string // XDG_CONFIG_HOME
	DataHome   string // XDG_DATA_HOME
	StateHome  string // XDG_STATE_HOME
	CacheHome  string // XDG_CACHE_HOME
}

// BaseDirs holds the resolved absolute base directory of every XDG location.
// Its zero value is not usable; Resolve is the only way to construct one.
type BaseDirs struct {
	config string
	data   string
	state  string
	cache  string
}

// Resolve returns the base directory of every XDG location, derived from
// environment. It fails with ErrInvalidHome unless the home directory is
// absolute, since every default is resolved against it.
func Resolve(environment Environment) (BaseDirs, error) {
	homePath := environment.Home

	if !filepath.IsAbs(homePath) {
		return BaseDirs{}, fmt.Errorf(
			"resolve base directories from %q: %w", homePath, ErrInvalidHome,
		)
	}

	localPath := filepath.Join(homePath, ".local")

	return BaseDirs{
		config: base(environment.ConfigHome, filepath.Join(homePath, ".config")),
		data:   base(environment.DataHome, filepath.Join(localPath, "share")),
		state:  base(environment.StateHome, filepath.Join(localPath, "state")),
		cache:  base(environment.CacheHome, filepath.Join(homePath, ".cache")),
	}, nil
}

// base returns the base directory a location resolves to: the path its variable
// specifies, or the specification's default when that variable is unusable. A
// relative path is unusable because the specification declares it invalid;
// likewise the filesystem root, because it is no place to relocate an entry to.
func base(candidatePath string, fallbackPath string) string {
	basePath := filepath.Clean(candidatePath)
	isRoot := basePath == string(filepath.Separator)

	if !filepath.IsAbs(basePath) || isRoot {
		return fallbackPath
	}

	return basePath
}

// Target is the absolute path an Entry resolves to, derived from its location
// and the path its rule gives relative to that location's base.
type Target string

// Target resolves relPath, a path relative to location's base directory, into
// the absolute target it specifies. It fails with ErrInvalidTarget unless
// relPath specifies something inside that base directory, and panics on a
// location this package does not define.
func (b BaseDirs) Target(relPath string, location Location) (Target, error) {
	basePath := b.BaseDir(location)
	targetPath := filepath.Join(basePath, relPath)
	insideBase := !filepath.IsAbs(relPath) && within(targetPath, basePath)

	if !insideBase {
		return "", fmt.Errorf("resolve %q: %w", relPath, ErrInvalidTarget)
	}

	return Target(targetPath), nil
}

// BaseDir returns the absolute base directory that location resolves to. It
// panics on a location this package does not define.
func (b BaseDirs) BaseDir(location Location) string {
	switch location {
	case LocationConfig:
		return b.config

	case LocationData:
		return b.data

	case LocationState:
		return b.state

	case LocationCache:
		return b.cache

	default:
		panic(fmt.Sprintf("undefined location %q", location))
	}
}

// within reports whether path exists inside dir. A path equal to dir is not
// within it, since a target is something dir contains, never dir itself.
func within(path string, dir string) bool {
	return strings.HasPrefix(path, dir+string(filepath.Separator))
}
