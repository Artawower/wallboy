// Package state handles persistent state management for wallboy.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CurrentWallpaper represents the currently set wallpaper.
type CurrentWallpaper struct {
	Path     string    `json:"path"`
	SourceID string    `json:"source_id"`
	Theme    string    `json:"theme"`
	SetAt    time.Time `json:"set_at"`
	IsTemp   bool      `json:"is_temp,omitempty"` // true if image is in temp dir (not saved)
	Query    string    `json:"query,omitempty"`   // Query used to fetch this image (for remote sources)
}

// PrefetchEntry represents a prefetched wallpaper ready to be used.
type PrefetchEntry struct {
	Path      string    `json:"path"`            // Path to prefetched image in temp dir
	FetchedAt time.Time `json:"fetched_at"`      // When it was fetched
	Query     string    `json:"query,omitempty"` // Query used to fetch this image
}

// State represents the persistent state.
type State struct {
	Theme      string                    `json:"theme"`
	Current    CurrentWallpaper          `json:"current"`
	History    []string                  `json:"history"`
	Prefetched map[string]*PrefetchEntry `json:"prefetched,omitempty"` // Prefetched per source ID

	// Runtime fields
	path string
}

// New creates a new empty state.
func New(path string) *State {
	return &State{
		path:    path,
		History: []string{},
	}
}

// legacyState is used to migrate from old state format.
type legacyState struct {
	Theme      string           `json:"theme"`
	Current    CurrentWallpaper `json:"current"`
	History    []string         `json:"history"`
	Prefetched json.RawMessage  `json:"prefetched,omitempty"` // Can be old or new format
}

// legacyPrefetchEntry is the old prefetch format (single entry with cache key).
type legacyPrefetchEntry struct {
	Path      string    `json:"path"`
	SourceID  string    `json:"source_id"`
	CacheKey  string    `json:"cache_key"`
	FetchedAt time.Time `json:"fetched_at"`
}

// Load loads state from a JSON file.
func Load(path string) (*State, error) {
	path = expandPath(path)

	s := &State{
		path:    path,
		History: []string{},
	}

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return s, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	// Handle empty file
	if len(data) == 0 {
		return s, nil
	}

	// First try to parse as legacy format to handle migration
	var legacy legacyState
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	// Copy basic fields
	s.Theme = legacy.Theme
	s.Current = legacy.Current
	s.History = legacy.History

	// Handle prefetched field migration
	if len(legacy.Prefetched) > 0 && string(legacy.Prefetched) != "null" {
		// Try to parse as new format (map)
		var newFormat map[string]*PrefetchEntry
		if err := json.Unmarshal(legacy.Prefetched, &newFormat); err == nil {
			s.Prefetched = newFormat
		} else {
			// Try to parse as old format (single entry)
			var oldFormat legacyPrefetchEntry
			if err := json.Unmarshal(legacy.Prefetched, &oldFormat); err == nil && oldFormat.Path != "" {
				// Migrate: use source_id as key, ignore cache_key
				s.Prefetched = map[string]*PrefetchEntry{
					oldFormat.SourceID: {
						Path:      oldFormat.Path,
						FetchedAt: oldFormat.FetchedAt,
					},
				}
			}
			// If both fail, just ignore the prefetched field (it will be nil)
		}
	}

	s.path = path
	return s, nil
}

// Save saves the state to a JSON file.
func (s *State) Save() error {
	if s.path == "" {
		return fmt.Errorf("state path not set")
	}

	// Ensure directory exists
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// SetCurrent sets the current wallpaper and updates history.
func (s *State) SetCurrent(path, sourceID, theme, query string, isTemp bool) {
	// Add previous to history if exists and it was saved (not temp)
	if s.Current.Path != "" && !s.Current.IsTemp {
		s.addToHistory(s.Current.Path)
	}

	s.Current = CurrentWallpaper{
		Path:     path,
		SourceID: sourceID,
		Theme:    theme,
		SetAt:    time.Now(),
		IsTemp:   isTemp,
		Query:    query,
	}
	s.Theme = theme
}

// MarkSaved marks current wallpaper as saved (not temp).
func (s *State) MarkSaved(newPath string) {
	s.Current.Path = newPath
	s.Current.IsTemp = false
}

// IsTempWallpaper returns true if current wallpaper is temporary.
func (s *State) IsTempWallpaper() bool {
	return s.Current.IsTemp
}

// addToHistory adds a path to history, maintaining max size.
func (s *State) addToHistory(path string) {
	const maxHistory = 100

	// Don't add duplicates
	for _, h := range s.History {
		if h == path {
			return
		}
	}

	s.History = append(s.History, path)

	// Trim if exceeds max
	if len(s.History) > maxHistory {
		s.History = s.History[len(s.History)-maxHistory:]
	}
}

// IsInHistory checks if a path is in history.
func (s *State) IsInHistory(path string) bool {
	for _, h := range s.History {
		if h == path {
			return true
		}
	}
	return false
}

// Clear clears the current wallpaper.
func (s *State) Clear() {
	s.Current = CurrentWallpaper{}
}

// HasCurrent returns true if there's a current wallpaper set.
func (s *State) HasCurrent() bool {
	return s.Current.Path != ""
}

// Path returns the state file path.
func (s *State) Path() string {
	return s.path
}

// GetPrefetchedForSource returns the prefetched entry for a specific source.
// Returns nil if no prefetch exists or file no longer exists.
func (s *State) GetPrefetchedForSource(sourceID string) *PrefetchEntry {
	if s.Prefetched == nil {
		return nil
	}
	entry, ok := s.Prefetched[sourceID]
	if !ok || entry == nil {
		return nil
	}
	// Check if file still exists
	if _, err := os.Stat(entry.Path); os.IsNotExist(err) {
		delete(s.Prefetched, sourceID)
		return nil
	}
	return entry
}

// SetPrefetchedForSource sets the prefetched wallpaper for a specific source.
func (s *State) SetPrefetchedForSource(sourceID, path, query string) {
	if s.Prefetched == nil {
		s.Prefetched = make(map[string]*PrefetchEntry)
	}
	s.Prefetched[sourceID] = &PrefetchEntry{
		Path:      path,
		FetchedAt: time.Now(),
		Query:     query,
	}
}

// ClearPrefetchedForSource clears the prefetched entry for a specific source.
func (s *State) ClearPrefetchedForSource(sourceID string) {
	if s.Prefetched != nil {
		delete(s.Prefetched, sourceID)
	}
}

// HasPrefetchedForSource returns true if there's a valid prefetched wallpaper for a source.
func (s *State) HasPrefetchedForSource(sourceID string) bool {
	return s.GetPrefetchedForSource(sourceID) != nil
}

// GetPrefetch implements datasource.PrefetchStore interface.
// Returns the prefetched image path and query for a source, if available.
func (s *State) GetPrefetch(sourceID string) (string, string, bool) {
	entry := s.GetPrefetchedForSource(sourceID)
	if entry == nil {
		return "", "", false
	}
	return entry.Path, entry.Query, true
}

// SetPrefetch implements datasource.PrefetchStore interface.
// Stores a prefetched image path and query for a source.
func (s *State) SetPrefetch(sourceID, path, query string) {
	s.SetPrefetchedForSource(sourceID, path, query)
}

// ClearPrefetch implements datasource.PrefetchStore interface.
// Removes the prefetched entry for a source.
func (s *State) ClearPrefetch(sourceID string) {
	s.ClearPrefetchedForSource(sourceID)
}

// expandPath expands ~ to home directory.
func expandPath(path string) string {
	if path == "" {
		return ""
	}
	if len(path) >= 2 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
