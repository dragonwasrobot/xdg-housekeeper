// Package rules contains the mapping of entries to apps with their relocation
// rules. Also contains the mapping of entries to known unsupported apps.
package rules

import (
	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/xdg"
)

// Rule captures where one app's entry belongs and how it gets there.
type Rule struct {
	App        string
	EntryName  string
	Location   xdg.Location
	RelPath    string
	Relocation Relocation
}

// Relocation specifies how a rule may relocate an entry it claims.
//
//sumtype:decl
type Relocation interface {
	isRelocation()
}

// RelocationMove moves the entry to its target, leaving nothing behind.
type RelocationMove struct{}

// isRelocation implements Relocation.
func (RelocationMove) isRelocation() {}

// RelocationSymlink moves the entry to its target and leaves a link at the old
// path.
type RelocationSymlink struct {
	// Note details what the user needs to do to make the link redundant.
	Note string
}

// isRelocation implements Relocation.
func (RelocationSymlink) isRelocation() {}

// Rules returns the rules list declaring one rule per entry claimed.
func Rules() []Rule {
	return []Rule{
		{
			App:        "git",
			EntryName:  ".gitconfig",
			Location:   xdg.LocationConfig,
			RelPath:    "git/config",
			Relocation: RelocationMove{},
		},
		{
			App:       "less",
			EntryName: ".lesshst",
			Location:  xdg.LocationState,
			RelPath:   "less/history",
			Relocation: RelocationSymlink{
				Note: "set LESSHISTFILE to make less use the new path " +
					"directly",
			},
		},
		{
			App:       "wget",
			EntryName: ".wgetrc",
			Location:  xdg.LocationConfig,
			RelPath:   "wget/wgetrc",
			Relocation: RelocationSymlink{
				Note: "set WGETRC to make wget use the new path directly",
			},
		},
		{
			App:       "gnupg",
			EntryName: ".gnupg",
			Location:  xdg.LocationData,
			RelPath:   "gnupg",
			Relocation: RelocationSymlink{
				Note: "set GNUPGHOME to make gpg use the new path directly",
			},
		},
		{
			App:       "aws",
			EntryName: ".aws",
			Location:  xdg.LocationConfig,
			RelPath:   "aws",
			Relocation: RelocationSymlink{
				Note: "set AWS_CONFIG_FILE and AWS_SHARED_CREDENTIALS_FILE " +
					"to make aws use the new paths directly",
			},
		},
	}
}

// UnsupportedEntry captures an entry an unsupported app owns, and the
// obstacle that keeps it in place.
type UnsupportedEntry struct {
	App      string
	Name     string
	Obstacle string
}

// UnsupportedEntries returns the unsupported entries list.
func UnsupportedEntries() []UnsupportedEntry {
	return []UnsupportedEntry{
		{
			App:      "bash",
			Name:     ".bashrc",
			Obstacle: "read from a fixed path before any variable applies",
		},
		{
			App:      "bash",
			Name:     ".bash_profile",
			Obstacle: "read from a fixed path before any variable applies",
		},
	}
}
