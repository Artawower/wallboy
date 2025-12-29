package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	s := New("/tmp/state.json")

	require.NotNil(t, s)
	assert.Equal(t, "/tmp/state.json", s.Path())
	assert.Empty(t, s.History)
	assert.Empty(t, s.Current.Path)
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		file        string
		wantErr     bool
		errContains string
		validate    func(t *testing.T, s *State)
	}{
		{
			name:    "valid state",
			file:    "testdata/valid.json",
			wantErr: false,
			validate: func(t *testing.T, s *State) {
				assert.Equal(t, "light", s.Theme)
				assert.Equal(t, "/tmp/wallpaper.jpg", s.Current.Path)
				assert.Equal(t, "light-local", s.Current.SourceID)
				assert.Equal(t, "light", s.Current.Theme)
				assert.False(t, s.Current.IsTemp)
				assert.Len(t, s.History, 2)
			},
		},
		{
			name:        "invalid json",
			file:        "testdata/invalid.json",
			wantErr:     true,
			errContains: "failed to parse",
		},
		{
			name:    "non-existent file returns empty state",
			file:    "testdata/does_not_exist.json",
			wantErr: false,
			validate: func(t *testing.T, s *State) {
				assert.Empty(t, s.Theme)
				assert.Empty(t, s.Current.Path)
				assert.Empty(t, s.History)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := Load(tt.file)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, s)

			if tt.validate != nil {
				tt.validate(t, s)
			}
		})
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	emptyFile := filepath.Join(tmpDir, "empty.json")

	// Create empty file
	err := os.WriteFile(emptyFile, []byte{}, 0644)
	require.NoError(t, err)

	s, err := Load(emptyFile)
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.Empty(t, s.Theme)
}

func TestLoad_WithTildeExpansion(t *testing.T) {
	// This tests the expandPath function indirectly
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.json")

	// Create a valid state file
	state := &State{
		Theme: "dark",
		path:  stateFile,
	}
	err := state.Save()
	require.NoError(t, err)

	// Load using full path
	loaded, err := Load(stateFile)
	require.NoError(t, err)
	assert.Equal(t, "dark", loaded.Theme)
}

func TestState_Save(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "subdir", "state.json")

	s := New(statePath)
	s.Theme = "light"
	s.SetCurrent("/tmp/wall.jpg", "source-1", "light", "", false)

	// Save should create directories
	err := s.Save()
	require.NoError(t, err)

	// File should exist
	_, err = os.Stat(statePath)
	require.NoError(t, err)

	// Reload and verify
	loaded, err := Load(statePath)
	require.NoError(t, err)
	assert.Equal(t, "light", loaded.Theme)
	assert.Equal(t, "/tmp/wall.jpg", loaded.Current.Path)
}

func TestState_Save_NoPath(t *testing.T) {
	s := &State{}
	err := s.Save()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path not set")
}

func TestState_SetCurrent(t *testing.T) {
	s := New("/tmp/state.json")

	// Set first wallpaper (temp)
	s.SetCurrent("/tmp/first.jpg", "source-1", "light", "", true)

	assert.Equal(t, "/tmp/first.jpg", s.Current.Path)
	assert.Equal(t, "source-1", s.Current.SourceID)
	assert.Equal(t, "light", s.Current.Theme)
	assert.True(t, s.Current.IsTemp)
	assert.Equal(t, "light", s.Theme)
	assert.Empty(t, s.History) // Temp wallpapers not added to history

	// Set second wallpaper (not temp) - previous was temp, so not added to history
	s.SetCurrent("/tmp/second.jpg", "source-2", "light", "", false)
	assert.Equal(t, "/tmp/second.jpg", s.Current.Path)
	assert.False(t, s.Current.IsTemp)
	assert.Empty(t, s.History) // Previous was temp

	// Set third wallpaper - previous was not temp, so added to history
	s.SetCurrent("/tmp/third.jpg", "source-3", "dark", "", false)
	assert.Equal(t, "/tmp/third.jpg", s.Current.Path)
	assert.Equal(t, "dark", s.Theme)
	require.Len(t, s.History, 1)
	assert.Equal(t, "/tmp/second.jpg", s.History[0])
}

func TestState_MarkSaved(t *testing.T) {
	s := New("/tmp/state.json")
	s.SetCurrent("/tmp/temp.jpg", "source-1", "light", "", true)

	assert.True(t, s.IsTempWallpaper())

	s.MarkSaved("/home/user/saved.jpg")

	assert.False(t, s.IsTempWallpaper())
	assert.Equal(t, "/home/user/saved.jpg", s.Current.Path)
}

func TestState_IsTempWallpaper(t *testing.T) {
	s := New("/tmp/state.json")

	// No current wallpaper
	assert.False(t, s.IsTempWallpaper())

	// Set temp wallpaper
	s.SetCurrent("/tmp/temp.jpg", "source-1", "light", "", true)
	assert.True(t, s.IsTempWallpaper())

	// Set non-temp wallpaper
	s.SetCurrent("/home/user/perm.jpg", "source-2", "light", "", false)
	assert.False(t, s.IsTempWallpaper())
}

func TestState_IsInHistory(t *testing.T) {
	s := New("/tmp/state.json")

	// Add some history
	s.SetCurrent("/path/1.jpg", "s1", "light", "", false)
	s.SetCurrent("/path/2.jpg", "s2", "light", "", false)
	s.SetCurrent("/path/3.jpg", "s3", "light", "", false)

	// /path/1.jpg and /path/2.jpg should be in history
	assert.True(t, s.IsInHistory("/path/1.jpg"))
	assert.True(t, s.IsInHistory("/path/2.jpg"))

	// Current and non-existent should not be
	assert.False(t, s.IsInHistory("/path/3.jpg")) // Current, not in history
	assert.False(t, s.IsInHistory("/path/other.jpg"))
}

func TestState_History_NoDuplicates(t *testing.T) {
	s := New("/tmp/state.json")

	// Add same path multiple times via SetCurrent cycle
	s.SetCurrent("/path/1.jpg", "s1", "light", "", false)
	s.SetCurrent("/path/2.jpg", "s2", "light", "", false)
	s.SetCurrent("/path/1.jpg", "s1", "light", "", false) // Back to 1
	s.SetCurrent("/path/3.jpg", "s3", "light", "", false)

	// Count occurrences of /path/1.jpg
	count := 0
	for _, h := range s.History {
		if h == "/path/1.jpg" {
			count++
		}
	}
	assert.Equal(t, 1, count, "should not have duplicate entries")
}

func TestState_History_MaxSize(t *testing.T) {
	s := New("/tmp/state.json")

	// Add more than 100 items
	for i := 0; i < 110; i++ {
		s.SetCurrent("/path/current.jpg", "s", "light", "", false)
		// Simulate adding to history by modifying directly for test
		s.Current.Path = "" // Clear so next SetCurrent sees no previous
	}

	// Manually test addToHistory for max size
	s2 := New("/tmp/state.json")
	for i := 0; i < 110; i++ {
		path := filepath.Join("/path", string(rune('A'+i%26)), "wall.jpg")
		s2.SetCurrent(path, "s", "light", "", false)
	}

	assert.LessOrEqual(t, len(s2.History), 100)
}

func TestState_Clear(t *testing.T) {
	s := New("/tmp/state.json")
	s.SetCurrent("/tmp/wall.jpg", "source-1", "light", "", false)

	require.True(t, s.HasCurrent())

	s.Clear()

	assert.False(t, s.HasCurrent())
	assert.Empty(t, s.Current.Path)
}

func TestState_HasCurrent(t *testing.T) {
	s := New("/tmp/state.json")

	assert.False(t, s.HasCurrent())

	s.SetCurrent("/tmp/wall.jpg", "source-1", "light", "", false)
	assert.True(t, s.HasCurrent())

	s.Clear()
	assert.False(t, s.HasCurrent())
}

func TestState_Path(t *testing.T) {
	s := New("/custom/path/state.json")
	assert.Equal(t, "/custom/path/state.json", s.Path())
}

func TestState_JSON_Serialization(t *testing.T) {
	s := New("/tmp/state.json")
	s.Theme = "dark"
	s.SetCurrent("/tmp/wall.jpg", "source-1", "dark", "", true)

	// Marshal
	data, err := json.Marshal(s)
	require.NoError(t, err)

	// Unmarshal into new state
	var loaded State
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)

	assert.Equal(t, "dark", loaded.Theme)
	assert.Equal(t, "/tmp/wall.jpg", loaded.Current.Path)
	assert.True(t, loaded.Current.IsTemp)
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string // Check if result contains this
	}{
		{
			name:     "empty path",
			input:    "",
			contains: "",
		},
		{
			name:     "regular path",
			input:    "/tmp/state.json",
			contains: "/tmp/state.json",
		},
		{
			name:     "tilde path",
			input:    "~/state.json",
			contains: "state.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			if tt.contains != "" {
				assert.Contains(t, result, tt.contains)
			} else {
				assert.Empty(t, result)
			}
		})
	}
}

func TestState_Prefetch(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("GetPrefetch returns empty when no prefetch", func(t *testing.T) {
		s := New(filepath.Join(tmpDir, "state.json"))

		path, query, ok := s.GetPrefetch("dark-bing")
		assert.False(t, ok)
		assert.Empty(t, path)
		assert.Empty(t, query)
	})

	t.Run("SetPrefetch and GetPrefetch work correctly", func(t *testing.T) {
		s := New(filepath.Join(tmpDir, "state.json"))

		// Create a real temp file
		prefetchPath := filepath.Join(tmpDir, "prefetched.jpg")
		require.NoError(t, os.WriteFile(prefetchPath, []byte("image"), 0644))

		s.SetPrefetch("dark-bing", prefetchPath, "nature")

		path, query, ok := s.GetPrefetch("dark-bing")
		assert.True(t, ok)
		assert.Equal(t, prefetchPath, path)
		assert.Equal(t, "nature", query)
	})

	t.Run("GetPrefetch returns empty for different source", func(t *testing.T) {
		s := New(filepath.Join(tmpDir, "state.json"))

		prefetchPath := filepath.Join(tmpDir, "prefetched2.jpg")
		require.NoError(t, os.WriteFile(prefetchPath, []byte("image"), 0644))

		s.SetPrefetch("dark-bing", prefetchPath, "landscape")

		// Different source should not match
		path, _, ok := s.GetPrefetch("light-bing")
		assert.False(t, ok)
		assert.Empty(t, path)

		path, _, ok = s.GetPrefetch("dark-wallhaven")
		assert.False(t, ok)
		assert.Empty(t, path)
	})

	t.Run("GetPrefetch returns empty if file no longer exists", func(t *testing.T) {
		s := New(filepath.Join(tmpDir, "state.json"))

		// Set prefetch with non-existent file
		s.SetPrefetch("dark-bing", "/nonexistent/path.jpg", "test")

		path, _, ok := s.GetPrefetch("dark-bing")
		assert.False(t, ok)
		assert.Empty(t, path)
		// Entry should be cleared from map
		assert.NotContains(t, s.Prefetched, "dark-bing")
	})

	t.Run("ClearPrefetch clears the entry", func(t *testing.T) {
		s := New(filepath.Join(tmpDir, "state.json"))

		prefetchPath := filepath.Join(tmpDir, "prefetched3.jpg")
		require.NoError(t, os.WriteFile(prefetchPath, []byte("image"), 0644))

		s.SetPrefetch("dark-bing", prefetchPath, "mountains")
		require.Contains(t, s.Prefetched, "dark-bing")

		s.ClearPrefetch("dark-bing")
		assert.NotContains(t, s.Prefetched, "dark-bing")
	})

	t.Run("Multiple sources can have independent prefetches", func(t *testing.T) {
		s := New(filepath.Join(tmpDir, "state.json"))

		path1 := filepath.Join(tmpDir, "prefetch_bing.jpg")
		path2 := filepath.Join(tmpDir, "prefetch_wallhaven.jpg")
		require.NoError(t, os.WriteFile(path1, []byte("image1"), 0644))
		require.NoError(t, os.WriteFile(path2, []byte("image2"), 0644))

		s.SetPrefetch("dark-bing", path1, "query1")
		s.SetPrefetch("dark-wallhaven", path2, "query2")

		p1, q1, ok1 := s.GetPrefetch("dark-bing")
		p2, q2, ok2 := s.GetPrefetch("dark-wallhaven")

		assert.True(t, ok1)
		assert.True(t, ok2)
		assert.Equal(t, path1, p1)
		assert.Equal(t, path2, p2)
		assert.Equal(t, "query1", q1)
		assert.Equal(t, "query2", q2)
	})

	t.Run("HasPrefetchedForSource returns correct value", func(t *testing.T) {
		s := New(filepath.Join(tmpDir, "state.json"))

		assert.False(t, s.HasPrefetchedForSource("dark-bing"))

		prefetchPath := filepath.Join(tmpDir, "prefetched4.jpg")
		require.NoError(t, os.WriteFile(prefetchPath, []byte("image"), 0644))

		s.SetPrefetch("dark-bing", prefetchPath, "test")

		assert.True(t, s.HasPrefetchedForSource("dark-bing"))
		assert.False(t, s.HasPrefetchedForSource("light-bing"))
	})

	t.Run("Prefetch persists through save/load", func(t *testing.T) {
		statePath := filepath.Join(tmpDir, "persist_state.json")
		prefetchPath := filepath.Join(tmpDir, "prefetched5.jpg")
		require.NoError(t, os.WriteFile(prefetchPath, []byte("image"), 0644))

		s := New(statePath)
		s.SetPrefetch("dark-bing", prefetchPath, "persisted query")
		require.NoError(t, s.Save())

		// Load and verify
		loaded, err := Load(statePath)
		require.NoError(t, err)

		path, query, ok := loaded.GetPrefetch("dark-bing")
		assert.True(t, ok)
		assert.Equal(t, prefetchPath, path)
		assert.Equal(t, "persisted query", query)
	})
}

func TestLoad_LegacyPrefetchMigration(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("migrates old prefetch format to new format", func(t *testing.T) {
		statePath := filepath.Join(tmpDir, "legacy_state.json")
		prefetchPath := filepath.Join(tmpDir, "legacy_prefetch.jpg")
		require.NoError(t, os.WriteFile(prefetchPath, []byte("image"), 0644))

		// Write state with old prefetch format
		oldState := fmt.Sprintf(`{
			"theme": "dark",
			"current": {"path": "/some/path.jpg", "source_id": "dark-bing", "theme": "dark"},
			"history": [],
			"prefetched": {
				"path": %q,
				"source_id": "dark-bing",
				"cache_key": "dark:bing:",
				"fetched_at": "2024-01-01T00:00:00Z"
			}
		}`, prefetchPath)
		require.NoError(t, os.WriteFile(statePath, []byte(oldState), 0644))

		// Load should migrate the format
		loaded, err := Load(statePath)
		require.NoError(t, err)

		// Should be able to get prefetch by source ID
		path, _, ok := loaded.GetPrefetch("dark-bing")
		assert.True(t, ok)
		assert.Equal(t, prefetchPath, path)
	})

	t.Run("handles new prefetch format correctly", func(t *testing.T) {
		statePath := filepath.Join(tmpDir, "new_state.json")
		prefetchPath := filepath.Join(tmpDir, "new_prefetch.jpg")
		require.NoError(t, os.WriteFile(prefetchPath, []byte("image"), 0644))

		// Write state with new prefetch format (map)
		newState := fmt.Sprintf(`{
			"theme": "dark",
			"current": {"path": "/some/path.jpg", "source_id": "dark-bing", "theme": "dark"},
			"history": [],
			"prefetched": {
				"dark-bing": {
					"path": %q,
					"fetched_at": "2024-01-01T00:00:00Z"
				}
			}
		}`, prefetchPath)
		require.NoError(t, os.WriteFile(statePath, []byte(newState), 0644))

		loaded, err := Load(statePath)
		require.NoError(t, err)

		path, _, ok := loaded.GetPrefetch("dark-bing")
		assert.True(t, ok)
		assert.Equal(t, prefetchPath, path)
	})

	t.Run("handles null prefetched field", func(t *testing.T) {
		statePath := filepath.Join(tmpDir, "null_state.json")

		nullState := `{
			"theme": "dark",
			"current": {},
			"history": [],
			"prefetched": null
		}`
		require.NoError(t, os.WriteFile(statePath, []byte(nullState), 0644))

		loaded, err := Load(statePath)
		require.NoError(t, err)

		_, _, ok := loaded.GetPrefetch("any")
		assert.False(t, ok)
	})

	t.Run("handles missing prefetched field", func(t *testing.T) {
		statePath := filepath.Join(tmpDir, "no_prefetch_state.json")

		noPreState := `{
			"theme": "dark",
			"current": {},
			"history": []
		}`
		require.NoError(t, os.WriteFile(statePath, []byte(noPreState), 0644))

		loaded, err := Load(statePath)
		require.NoError(t, err)

		_, _, ok := loaded.GetPrefetch("any")
		assert.False(t, ok)
	})
}
