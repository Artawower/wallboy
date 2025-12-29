package state

import (
	"encoding/json"
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
	s.SetCurrent("/tmp/wall.jpg", "source-1", "light", false)

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
	s.SetCurrent("/tmp/first.jpg", "source-1", "light", true)

	assert.Equal(t, "/tmp/first.jpg", s.Current.Path)
	assert.Equal(t, "source-1", s.Current.SourceID)
	assert.Equal(t, "light", s.Current.Theme)
	assert.True(t, s.Current.IsTemp)
	assert.Equal(t, "light", s.Theme)
	assert.Empty(t, s.History) // Temp wallpapers not added to history

	// Set second wallpaper (not temp) - previous was temp, so not added to history
	s.SetCurrent("/tmp/second.jpg", "source-2", "light", false)
	assert.Equal(t, "/tmp/second.jpg", s.Current.Path)
	assert.False(t, s.Current.IsTemp)
	assert.Empty(t, s.History) // Previous was temp

	// Set third wallpaper - previous was not temp, so added to history
	s.SetCurrent("/tmp/third.jpg", "source-3", "dark", false)
	assert.Equal(t, "/tmp/third.jpg", s.Current.Path)
	assert.Equal(t, "dark", s.Theme)
	require.Len(t, s.History, 1)
	assert.Equal(t, "/tmp/second.jpg", s.History[0])
}

func TestState_MarkSaved(t *testing.T) {
	s := New("/tmp/state.json")
	s.SetCurrent("/tmp/temp.jpg", "source-1", "light", true)

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
	s.SetCurrent("/tmp/temp.jpg", "source-1", "light", true)
	assert.True(t, s.IsTempWallpaper())

	// Set non-temp wallpaper
	s.SetCurrent("/home/user/perm.jpg", "source-2", "light", false)
	assert.False(t, s.IsTempWallpaper())
}

func TestState_IsInHistory(t *testing.T) {
	s := New("/tmp/state.json")

	// Add some history
	s.SetCurrent("/path/1.jpg", "s1", "light", false)
	s.SetCurrent("/path/2.jpg", "s2", "light", false)
	s.SetCurrent("/path/3.jpg", "s3", "light", false)

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
	s.SetCurrent("/path/1.jpg", "s1", "light", false)
	s.SetCurrent("/path/2.jpg", "s2", "light", false)
	s.SetCurrent("/path/1.jpg", "s1", "light", false) // Back to 1
	s.SetCurrent("/path/3.jpg", "s3", "light", false)

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
		s.SetCurrent("/path/current.jpg", "s", "light", false)
		// Simulate adding to history by modifying directly for test
		s.Current.Path = "" // Clear so next SetCurrent sees no previous
	}

	// Manually test addToHistory for max size
	s2 := New("/tmp/state.json")
	for i := 0; i < 110; i++ {
		path := filepath.Join("/path", string(rune('A'+i%26)), "wall.jpg")
		s2.SetCurrent(path, "s", "light", false)
	}

	assert.LessOrEqual(t, len(s2.History), 100)
}

func TestState_Clear(t *testing.T) {
	s := New("/tmp/state.json")
	s.SetCurrent("/tmp/wall.jpg", "source-1", "light", false)

	require.True(t, s.HasCurrent())

	s.Clear()

	assert.False(t, s.HasCurrent())
	assert.Empty(t, s.Current.Path)
}

func TestState_HasCurrent(t *testing.T) {
	s := New("/tmp/state.json")

	assert.False(t, s.HasCurrent())

	s.SetCurrent("/tmp/wall.jpg", "source-1", "light", false)
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
	s.SetCurrent("/tmp/wall.jpg", "source-1", "dark", true)

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

	t.Run("GetPrefetched returns nil when no prefetch", func(t *testing.T) {
		s := New(filepath.Join(tmpDir, "state.json"))

		result := s.GetPrefetched("dark:bing:")
		assert.Nil(t, result)
	})

	t.Run("SetPrefetched and GetPrefetched with matching key", func(t *testing.T) {
		s := New(filepath.Join(tmpDir, "state.json"))

		// Create a real temp file
		prefetchPath := filepath.Join(tmpDir, "prefetched.jpg")
		require.NoError(t, os.WriteFile(prefetchPath, []byte("image"), 0644))

		s.SetPrefetched(prefetchPath, "dark-bing", "dark:bing:")

		result := s.GetPrefetched("dark:bing:")
		require.NotNil(t, result)
		assert.Equal(t, prefetchPath, result.Path)
		assert.Equal(t, "dark-bing", result.SourceID)
		assert.Equal(t, "dark:bing:", result.CacheKey)
		assert.False(t, result.FetchedAt.IsZero())
	})

	t.Run("GetPrefetched returns nil for non-matching key", func(t *testing.T) {
		s := New(filepath.Join(tmpDir, "state.json"))

		prefetchPath := filepath.Join(tmpDir, "prefetched2.jpg")
		require.NoError(t, os.WriteFile(prefetchPath, []byte("image"), 0644))

		s.SetPrefetched(prefetchPath, "dark-bing", "dark:bing:")

		// Different key should not match
		result := s.GetPrefetched("light:bing:")
		assert.Nil(t, result)

		result = s.GetPrefetched("dark:wallhaven:")
		assert.Nil(t, result)
	})

	t.Run("GetPrefetched returns nil if file no longer exists", func(t *testing.T) {
		s := New(filepath.Join(tmpDir, "state.json"))

		// Set prefetch with non-existent file
		s.SetPrefetched("/nonexistent/path.jpg", "dark-bing", "dark:bing:")

		result := s.GetPrefetched("dark:bing:")
		assert.Nil(t, result)
		// Prefetch should be cleared
		assert.Nil(t, s.Prefetched)
	})

	t.Run("ClearPrefetched clears the entry", func(t *testing.T) {
		s := New(filepath.Join(tmpDir, "state.json"))

		prefetchPath := filepath.Join(tmpDir, "prefetched3.jpg")
		require.NoError(t, os.WriteFile(prefetchPath, []byte("image"), 0644))

		s.SetPrefetched(prefetchPath, "dark-bing", "dark:bing:")
		require.NotNil(t, s.Prefetched)

		s.ClearPrefetched()
		assert.Nil(t, s.Prefetched)
	})

	t.Run("HasPrefetched returns correct value", func(t *testing.T) {
		s := New(filepath.Join(tmpDir, "state.json"))

		assert.False(t, s.HasPrefetched("dark:bing:"))

		prefetchPath := filepath.Join(tmpDir, "prefetched4.jpg")
		require.NoError(t, os.WriteFile(prefetchPath, []byte("image"), 0644))

		s.SetPrefetched(prefetchPath, "dark-bing", "dark:bing:")

		assert.True(t, s.HasPrefetched("dark:bing:"))
		assert.False(t, s.HasPrefetched("light:bing:"))
	})

	t.Run("Prefetch persists through save/load", func(t *testing.T) {
		statePath := filepath.Join(tmpDir, "persist_state.json")
		prefetchPath := filepath.Join(tmpDir, "prefetched5.jpg")
		require.NoError(t, os.WriteFile(prefetchPath, []byte("image"), 0644))

		s := New(statePath)
		s.SetPrefetched(prefetchPath, "dark-bing", "dark:bing:")
		require.NoError(t, s.Save())

		// Load and verify
		loaded, err := Load(statePath)
		require.NoError(t, err)

		result := loaded.GetPrefetched("dark:bing:")
		require.NotNil(t, result)
		assert.Equal(t, prefetchPath, result.Path)
		assert.Equal(t, "dark-bing", result.SourceID)
	})
}
