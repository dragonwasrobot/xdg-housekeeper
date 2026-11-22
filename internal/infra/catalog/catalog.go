// Package catalog is the rule table adapter: reading the rule table from a
// directory holding one JSON file per app.
package catalog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"

	"github.com/samber/lo"

	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/rules"
	"github.com/dragonwasrobot/xdg-housekeeper/internal/domain/xdg"
)

// ErrUnknownRelocation reports that an app file names a relocation other than
// a move or a symlink.
var ErrUnknownRelocation = errors.New("unknown relocation")

// ErrUnknownLocation reports that an app file names a location the XDG
// specification does not define.
var ErrUnknownLocation = errors.New("unknown location")

// ErrNoteOnMove reports that an app file gives a move a note, which only a
// symlink carries.
var ErrNoteOnMove = errors.New("note on a move")

// ErrMalformedAppFile reports that an app file is not a JSON document in the
// app file format.
var ErrMalformedAppFile = errors.New("malformed app file")

// ErrMissingApp reports that an app file does not name the app its
// declarations belong to.
var ErrMissingApp = errors.New("missing app")

// Load reads the rule table from a directory holding one JSON file per app.
func Load(dir fs.FS) (rules.Table, error) {
	// appFilePattern matches the app files in a catalog directory.
	const appFilePattern = "*.json"

	paths, err := fs.Glob(dir, appFilePattern)
	if err != nil {
		// Glob fails only on a malformed pattern, and this one is fixed.
		panic(fmt.Sprintf("glob %q: %v", appFilePattern, err))
	}

	var (
		declaredRules       []rules.Rule
		declaredUnsupported []rules.UnsupportedEntry
	)

	for _, path := range paths {
		appRules, appUnsupported, err := loadAppFile(dir, path)
		if err != nil {
			return rules.Table{}, fmt.Errorf("app file %q: %w", path, err)
		}

		declaredRules = append(declaredRules, appRules...)
		declaredUnsupported = append(declaredUnsupported, appUnsupported...)
	}

	return rules.BuildTable(declaredRules, declaredUnsupported)
}

// appFile is one app file as it is written: the app, the rules for its
// entries, and the entries it owns that cannot be relocated.
type appFile struct {
	App         string            `json:"app"`
	Rules       []ruleJSON        `json:"rules"`
	Unsupported []unsupportedJSON `json:"unsupported"`
}

// ruleJSON is one rule as an app file writes it.
type ruleJSON struct {
	EntryName  string         `json:"entryName"`
	Location   string         `json:"location"`
	RelPath    string         `json:"relPath"`
	Relocation relocationJSON `json:"relocation"`
}

// relocationJSON is one relocation as an app file writes it: its kind, and
// the note a symlink carries.
type relocationJSON struct {
	Kind string `json:"kind"`
	Note string `json:"note"`
}

// unsupportedJSON is one unsupported entry as an app file writes it.
type unsupportedJSON struct {
	EntryName string `json:"entryName"`
	Obstacle  string `json:"obstacle"`
}

// loadAppFile reads the rules and unsupported entries one app file declares.
func loadAppFile(
	dir fs.FS, path string,
) ([]rules.Rule, []rules.UnsupportedEntry, error) {
	file, err := readAppFile(dir, path)
	if err != nil {
		return nil, nil, err
	}

	declared, err := rulesFrom(file)
	if err != nil {
		return nil, nil, err
	}

	return declared, unsupportedFrom(file), nil
}

// readAppFile reads the app file at path as it is written.
func readAppFile(dir fs.FS, path string) (appFile, error) {
	data, err := fs.ReadFile(dir, path)
	if err != nil {
		//nolint:errorlint // a read error is a stdlib detail
		return appFile{}, fmt.Errorf("read: %v", err)
	}

	return decodeAppFile(data)
}

// decodeAppFile decodes data as exactly one app file, refusing a key the
// format does not define or a file naming no app.
func decodeAppFile(data []byte) (appFile, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var file appFile
	if err := decoder.Decode(&file); err != nil {
		//nolint:errorlint // a decode error is a stdlib detail
		return appFile{}, fmt.Errorf("%w: %v", ErrMalformedAppFile, err)
	}

	ended := errors.Is(decoder.Decode(&struct{}{}), io.EOF)

	if !ended {
		return appFile{}, fmt.Errorf("%w: content after the document",
			ErrMalformedAppFile)
	}

	if file.App == "" {
		return appFile{}, ErrMissingApp
	}

	return file, nil
}

// rulesFrom returns the rules an app file writes.
func rulesFrom(file appFile) ([]rules.Rule, error) {
	declared := make([]rules.Rule, 0, len(file.Rules))

	for _, raw := range file.Rules {
		rule, err := ruleFrom(file.App, raw)
		if err != nil {
			return nil, fmt.Errorf("rule for %q: %w", raw.EntryName, err)
		}

		declared = append(declared, rule)
	}

	return declared, nil
}

// ruleFrom returns the rule an app file writes for the specified app.
func ruleFrom(app string, raw ruleJSON) (rules.Rule, error) {
	location, err := locationFrom(raw.Location)
	if err != nil {
		return rules.Rule{}, err
	}

	relocation, err := relocationFrom(raw.Relocation)
	if err != nil {
		return rules.Rule{}, err
	}

	return rules.Rule{
		App:        app,
		EntryName:  raw.EntryName,
		Location:   location,
		RelPath:    raw.RelPath,
		Relocation: relocation,
	}, nil
}

// locationFrom returns the location an app file writes, refusing one the XDG
// specification does not define.
func locationFrom(raw string) (xdg.Location, error) {
	location := xdg.Location(raw)
	defined := slices.Contains(xdg.Locations(), location)

	if !defined {
		return "", fmt.Errorf("%w %q", ErrUnknownLocation, raw)
	}

	return location, nil
}

// relocationFrom returns the relocation an app file writes, refusing a kind
// other than a move or a symlink.
func relocationFrom(raw relocationJSON) (rules.Relocation, error) {
	switch raw.Kind {
	case "move":
		if raw.Note != "" {
			return nil, ErrNoteOnMove
		}

		return rules.RelocationMove{}, nil

	case "symlink":
		return rules.RelocationSymlink{Note: raw.Note}, nil

	default:
		return nil, fmt.Errorf("%w %q", ErrUnknownRelocation, raw.Kind)
	}
}

// unsupportedFrom returns the unsupported entries an app file writes.
func unsupportedFrom(file appFile) []rules.UnsupportedEntry {
	return lo.Map(file.Unsupported,
		func(raw unsupportedJSON, _ int) rules.UnsupportedEntry {
			return rules.UnsupportedEntry{
				App:       file.App,
				EntryName: raw.EntryName,
				Obstacle:  raw.Obstacle,
			}
		})
}
