package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// listPrefs are the interactive list's remembered settings, saved when a sort
// is applied from the "sort by" menu and restored the next time it opens
// without --sort-by/--ascending. Only the interactive view reads them: plain
// `list` output stays a function of its flags, so scripts don't change behavior
// with whoever last used the TUI.
type listPrefs struct {
	SortBy    string `json:"sortBy"`
	Ascending bool   `json:"ascending"`
}

// prefsPath is <user config dir>/oc-aibom/prefs.json ($XDG_CONFIG_HOME or
// ~/.config on Linux, ~/Library/Application Support on macOS).
func prefsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "oc-aibom", "prefs.json"), nil
}

// loadListPrefs returns the saved prefs; ok is false when there are none or
// they're unreadable or name a sort the menu no longer offers.
func loadListPrefs() (p listPrefs, ok bool) {
	path, err := prefsPath()
	if err != nil {
		return p, false
	}
	b, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(b, &p) != nil {
		return p, false
	}
	for _, s := range sortOptions {
		if s == p.SortBy {
			return p, true
		}
	}
	return p, false
}

// saveListPrefs writes p. A failure is ignored: it's a convenience, and the
// sort itself has already been applied.
func saveListPrefs(p listPrefs) {
	path, err := prefsPath()
	if err != nil {
		return
	}
	b, _ := json.Marshal(p)
	if os.MkdirAll(filepath.Dir(path), 0o755) == nil {
		_ = os.WriteFile(path, b, 0o644)
	}
}
