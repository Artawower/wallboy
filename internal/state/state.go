package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type CurrentWallpaper struct {
	Path     string    `json:"path"`
	SourceID string    `json:"source_id"`
	Theme    string    `json:"theme"`
	SetAt    time.Time `json:"set_at"`
	IsTemp   bool      `json:"is_temp,omitempty"`
	Query    string    `json:"query,omitempty"`
}

type PrefetchEntry struct {
	Path      string    `json:"path"`
	FetchedAt time.Time `json:"fetched_at"`
	Query     string    `json:"query,omitempty"`
}

type State struct {
	Theme      string                    `json:"theme"`
	Current    CurrentWallpaper          `json:"current"`
	History    []string                  `json:"history"`
	Prefetched map[string]*PrefetchEntry `json:"prefetched,omitempty"`

	path string
}

func New(path string) *State {
	return &State{
		path:    path,
		History: []string{},
	}
}

type legacyState struct {
	Theme      string           `json:"theme"`
	Current    CurrentWallpaper `json:"current"`
	History    []string         `json:"history"`
	Prefetched json.RawMessage  `json:"prefetched,omitempty"`
}

type legacyPrefetchEntry struct {
	Path      string    `json:"path"`
	SourceID  string    `json:"source_id"`
	CacheKey  string    `json:"cache_key"`
	FetchedAt time.Time `json:"fetched_at"`
}

func Load(path string) (*State, error) {
	path = expandPath(path)

	s := &State{
		path:    path,
		History: []string{},
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return s, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	if len(data) == 0 {
		return s, nil
	}

	var legacy legacyState
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	s.Theme = legacy.Theme
	s.Current = legacy.Current
	s.History = legacy.History

	if len(legacy.Prefetched) > 0 && string(legacy.Prefetched) != "null" {
		var newFormat map[string]*PrefetchEntry
		if err := json.Unmarshal(legacy.Prefetched, &newFormat); err == nil {
			s.Prefetched = newFormat
		} else {
			var oldFormat legacyPrefetchEntry
			if err := json.Unmarshal(legacy.Prefetched, &oldFormat); err == nil && oldFormat.Path != "" {
				s.Prefetched = map[string]*PrefetchEntry{
					oldFormat.SourceID: {
						Path:      oldFormat.Path,
						FetchedAt: oldFormat.FetchedAt,
					},
				}
			}
		}
	}

	s.path = path
	return s, nil
}

func (s *State) Save() error {
	if s.path == "" {
		return fmt.Errorf("state path not set")
	}

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

func (s *State) SetCurrent(path, sourceID, theme, query string, isTemp bool) {
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

func (s *State) MarkSaved(newPath string) {
	s.Current.Path = newPath
	s.Current.IsTemp = false
}

func (s *State) IsTempWallpaper() bool {
	return s.Current.IsTemp
}

func (s *State) addToHistory(path string) {
	const maxHistory = 100

	for _, h := range s.History {
		if h == path {
			return
		}
	}

	s.History = append(s.History, path)

	if len(s.History) > maxHistory {
		s.History = s.History[len(s.History)-maxHistory:]
	}
}

func (s *State) IsInHistory(path string) bool {
	for _, h := range s.History {
		if h == path {
			return true
		}
	}
	return false
}

func (s *State) Clear() {
	s.Current = CurrentWallpaper{}
}

func (s *State) HasCurrent() bool {
	return s.Current.Path != ""
}

func (s *State) Path() string {
	return s.path
}

func (s *State) GetPrefetchedForSource(sourceID string) *PrefetchEntry {
	if s.Prefetched == nil {
		return nil
	}
	entry, ok := s.Prefetched[sourceID]
	if !ok || entry == nil {
		return nil
	}
	if _, err := os.Stat(entry.Path); os.IsNotExist(err) {
		delete(s.Prefetched, sourceID)
		return nil
	}
	return entry
}

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

func (s *State) ClearPrefetchedForSource(sourceID string) {
	if s.Prefetched != nil {
		delete(s.Prefetched, sourceID)
	}
}

func (s *State) HasPrefetchedForSource(sourceID string) bool {
	return s.GetPrefetchedForSource(sourceID) != nil
}

func (s *State) GetPrefetch(sourceID string) (string, string, bool) {
	entry := s.GetPrefetchedForSource(sourceID)
	if entry == nil {
		return "", "", false
	}
	return entry.Path, entry.Query, true
}

func (s *State) SetPrefetch(sourceID, path, query string) {
	s.SetPrefetchedForSource(sourceID, path, query)
}

func (s *State) ClearPrefetch(sourceID string) {
	s.ClearPrefetchedForSource(sourceID)
}

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
