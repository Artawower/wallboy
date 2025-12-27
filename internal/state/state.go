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
}

// State represents the persistent state.
type State struct {
	Theme   string           `json:"theme"`
	Current CurrentWallpaper `json:"current"`
	History []string         `json:"history"`

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

	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
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
func (s *State) SetCurrent(path, sourceID, theme string, isTemp bool) {
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
