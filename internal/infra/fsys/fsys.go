// Package fsys is the filesystem adapter: traversing home for the entries it
// holds, probing what stands in the way of a target, and relocating an entry
// to the target it belongs at.
package fsys

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/samber/lo"
)

// Entry is one dot-prefixed file or directory sitting directly in home,
// specified by an absolute path.
type Entry struct {
	Path string
}

// Traverse scans the home directory for entries, filtering out the specified
// base directory paths.
func Traverse(homePath string, baseDirPaths []string) ([]Entry, error) {
	dirEntries, err := os.ReadDir(homePath)
	if err != nil {
		//nolint:errorlint // a *os.PathError is an infra detail
		return nil, fmt.Errorf("traverse home: %v", err)
	}

	entryFor := func(dirEntry os.DirEntry, _ int) (Entry, bool) {
		entryPath := filepath.Join(homePath, dirEntry.Name())

		if !isEntry(entryPath, baseDirPaths) {
			return Entry{}, false
		}

		return Entry{Path: entryPath}, true
	}

	return lo.FilterMap(dirEntries, entryFor), nil
}

// isEntry reports whether a path in home is an entry. A path that is a base
// directory, or that holds one below it, is not.
func isEntry(entryPath string, baseDirPaths []string) bool {
	if !strings.HasPrefix(filepath.Base(entryPath), ".") {
		return false
	}

	holdsBaseDir := func(baseDirPath string) bool {
		return baseDirPath == entryPath ||
			strings.HasPrefix(baseDirPath, entryPath+string(filepath.Separator))
	}

	return !slices.ContainsFunc(baseDirPaths, holdsBaseDir)
}

// TargetState is what a probe found standing between an entry and its target.
//
//sumtype:decl
type TargetState interface {
	isTargetState()
}

// TargetOccupied reports that something already exists at the target.
type TargetOccupied struct{}

// isTargetState implements TargetState.
func (TargetOccupied) isTargetState() {}

// TargetBlocked reports that a non-directory sits where one of the target's
// parent directories would need to be.
type TargetBlocked struct {
	blocker string
}

// isTargetState implements TargetState.
func (TargetBlocked) isTargetState() {}

// Blocker returns the path of the file standing in the target's way.
func (t TargetBlocked) Blocker() string { return t.blocker }

// Probe reports what stands in the way of relocating to each of the specified
// targets. A target the result does not name is free.
func Probe(targetPaths []string) (map[string]TargetState, error) {
	states := make(map[string]TargetState, len(targetPaths))

	for _, targetPath := range targetPaths {
		// Check using Lstat to account for dangling symlinks.
		_, err := os.Lstat(targetPath)
		if err == nil {
			states[targetPath] = TargetOccupied{}

			continue
		}

		// Anything but an absent target is an error to report.
		if !isAbsent(err) {
			//nolint:errorlint // a *os.PathError is an infra detail
			return nil, fmt.Errorf("probe target: %v", err)
		}

		// Check if a non-directory on the path is blocking.
		blockerPath, blocked, err := blocker(targetPath)
		if err != nil {
			return nil, err
		}

		if blocked {
			states[targetPath] = TargetBlocked{blocker: blockerPath}
		}
	}

	return states, nil
}

// blocker returns the file standing between a target and the parent
// directories it needs, and whether there is one at all.
func blocker(targetPath string) (string, bool, error) {
	// The first component that exists decides the result: nothing can exist
	// under a regular file, so a directory here means every component above it
	// is one too. Stat used to capture the case where the entry can be
	// relocated through it, e.g. a symlink pointing at a directory.
	for dir := filepath.Dir(targetPath); ; dir = filepath.Dir(dir) {
		info, err := os.Stat(dir)
		if err == nil {
			if info.IsDir() {
				return "", false, nil
			}

			return dir, true, nil
		}

		// Anything but an absent parent is an error to report.
		if !isAbsent(err) {
			//nolint:errorlint // a *os.PathError is an infra detail
			return "", false, fmt.Errorf("probe target parent: %v", err)
		}

		// Stat calls a dangling symlink absent, but the name it holds is one
		// no directory can be created under.
		if _, err := os.Lstat(dir); err == nil {
			return dir, true, nil
		}

		// Check if reached root.
		isRoot := filepath.Dir(dir) == dir
		if isRoot {
			return "", false, nil
		}
	}
}

// isAbsent reports whether a probe error means nothing exists at the target.
func isAbsent(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

// Symlink relocates the entry to its target and leaves a symlink at the old
// path, so an app that cannot be pointed at the new location keeps working.
func Symlink(entryPath, targetPath string) error {
	if err := Move(entryPath, targetPath); err != nil {
		return err
	}

	// Reversed against this package's order: os.Symlink names the target
	// first, as `ln -s TARGET LINK_NAME` does.
	if err := os.Symlink(targetPath, entryPath); err != nil {
		//nolint:errorlint // a *os.LinkError is an infra detail
		return fmt.Errorf("entry relocated to %q, but linking back failed: %v",
			targetPath, err)
	}

	return nil
}

// Move relocates the entry to its target, creating the parent directories the
// target needs. Nothing is left behind at the entry's old path.
func Move(entryPath, targetPath string) error {
	// The XDG specification asks for 0700 on any directory created, and for an
	// existing one's mode to be left alone; MkdirAll does both.
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
		//nolint:errorlint // a *os.PathError is an infra detail
		return fmt.Errorf("create target parent: %v", err)
	}

	if err := os.Rename(entryPath, targetPath); err != nil {
		//nolint:errorlint // a *os.PathError is an infra detail
		return fmt.Errorf("relocate entry: %v", err)
	}

	return nil
}
