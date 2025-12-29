package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Artawower/wallboy/internal/config"
	"github.com/Artawower/wallboy/internal/datasource"
	"github.com/Artawower/wallboy/internal/platform"
	"github.com/Artawower/wallboy/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPlatform struct {
	wallpaperPath string
	wallpaperErr  error
	revealCalled  bool
	openCalled    bool
}

func (m *mockPlatform) Name() string                               { return "mock" }
func (m *mockPlatform) IsSupported() bool                          { return true }
func (m *mockPlatform) Wallpaper() platform.WallpaperService       { return m }
func (m *mockPlatform) Theme() platform.ThemeService               { return m }
func (m *mockPlatform) Scheduler() platform.SchedulerService       { return m }
func (m *mockPlatform) FileManager() platform.FileManagerService   { return m }
func (m *mockPlatform) Set(path string) error                      { return nil }
func (m *mockPlatform) Get() (string, error)                       { return m.wallpaperPath, m.wallpaperErr }
func (m *mockPlatform) Detect() platform.Theme                     { return platform.ThemeLight }
func (m *mockPlatform) Install(cfg platform.SchedulerConfig) error { return nil }
func (m *mockPlatform) Uninstall(label string) error               { return nil }
func (m *mockPlatform) Status(label string) (platform.SchedulerStatus, error) {
	return platform.SchedulerStatus{}, nil
}
func (m *mockPlatform) Reveal(path string) error {
	m.revealCalled = true
	return nil
}
func (m *mockPlatform) Open(path string) error {
	m.openCalled = true
	return nil
}

func TestColor_Hex(t *testing.T) {
	tests := []struct {
		name     string
		color    Color
		expected string
	}{
		{"black", Color{0, 0, 0}, "#000000"},
		{"white", Color{255, 255, 255}, "#ffffff"},
		{"red", Color{255, 0, 0}, "#ff0000"},
		{"green", Color{0, 255, 0}, "#00ff00"},
		{"blue", Color{0, 0, 255}, "#0000ff"},
		{"gray", Color{128, 128, 128}, "#808080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.color.Hex())
		})
	}
}

func TestTheme_ToConfigMode(t *testing.T) {
	assert.Equal(t, "light", string(ThemeLight.ToConfigMode()))
	assert.Equal(t, "dark", string(ThemeDark.ToConfigMode()))
}

func TestFromPlatformTheme(t *testing.T) {
	// Just verify the function works
	light := FromPlatformTheme("light")
	dark := FromPlatformTheme("dark")

	assert.Equal(t, ThemeLight, light)
	assert.Equal(t, ThemeDark, dark)
}

func TestThemeModeConstants(t *testing.T) {
	assert.Equal(t, ThemeMode("auto"), ThemeModeAuto)
	assert.Equal(t, ThemeMode("light"), ThemeModeLight)
	assert.Equal(t, ThemeMode("dark"), ThemeModeDark)
}

func TestThemeConstants(t *testing.T) {
	assert.Equal(t, Theme("light"), ThemeLight)
	assert.Equal(t, Theme("dark"), ThemeDark)
}

func TestWallpaperResult(t *testing.T) {
	now := time.Now()
	result := WallpaperResult{
		Path:     "/path/to/image.jpg",
		Theme:    "dark",
		SourceID: "wallhaven",
		IsTemp:   true,
		SetAt:    now,
	}

	assert.Equal(t, "/path/to/image.jpg", result.Path)
	assert.Equal(t, "dark", result.Theme)
	assert.Equal(t, "wallhaven", result.SourceID)
	assert.True(t, result.IsTemp)
	assert.Equal(t, now, result.SetAt)
}

func TestWallpaperInfo(t *testing.T) {
	now := time.Now()
	info := WallpaperInfo{
		Path:     "/path/to/image.jpg",
		Theme:    "light",
		SourceID: "local",
		IsTemp:   false,
		SetAt:    now,
		Exists:   true,
	}

	assert.Equal(t, "/path/to/image.jpg", info.Path)
	assert.Equal(t, "light", info.Theme)
	assert.Equal(t, "local", info.SourceID)
	assert.False(t, info.IsTemp)
	assert.Equal(t, now, info.SetAt)
	assert.True(t, info.Exists)
}

func TestSourceInfo(t *testing.T) {
	info := SourceInfo{
		ID:          "wallhaven-dark",
		Theme:       "dark",
		Type:        "remote",
		Description: "wallhaven",
	}

	assert.Equal(t, "wallhaven-dark", info.ID)
	assert.Equal(t, "dark", info.Theme)
	assert.Equal(t, "remote", info.Type)
	assert.Equal(t, "wallhaven", info.Description)
}

func TestAgentStatus(t *testing.T) {
	status := AgentStatus{
		Supported: true,
		Installed: true,
		Running:   true,
		Interval:  10 * time.Minute,
		LogPath:   "/var/log/agent.log",
	}

	assert.True(t, status.Supported)
	assert.True(t, status.Installed)
	assert.True(t, status.Running)
	assert.Equal(t, 10*time.Minute, status.Interval)
	assert.Equal(t, "/var/log/agent.log", status.LogPath)
}

func TestWithOptions(t *testing.T) {
	e := &Engine{}

	WithThemeOverride("dark")(e)
	assert.Equal(t, "dark", e.themeOverride)

	WithProviderOverride("wallhaven")(e)
	assert.Equal(t, "wallhaven", e.providerOverride)

	WithDryRun(true)(e)
	assert.True(t, e.dryRun)

	WithQueryOverride("nature landscape")(e)
	assert.Equal(t, "nature landscape", e.queryOverride)
}

func TestWithQueryOverride(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{"empty query", "", ""},
		{"single word", "nature", "nature"},
		{"multiple words", "nature landscape mountains", "nature landscape mountains"},
		{"with special chars", "dark wallpaper 4k", "dark wallpaper 4k"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Engine{}
			WithQueryOverride(tt.query)(e)
			assert.Equal(t, tt.expected, e.queryOverride)
		})
	}
}

func TestEngine_OpenInFinder(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("error when platform returns empty path", func(t *testing.T) {
		mock := &mockPlatform{wallpaperPath: ""}
		e := &Engine{platform: mock}

		err := e.OpenInFinder()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no wallpaper path available")
	})

	t.Run("error when file does not exist", func(t *testing.T) {
		mock := &mockPlatform{wallpaperPath: "/nonexistent/wallpaper.jpg"}
		e := &Engine{platform: mock}

		err := e.OpenInFinder()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no longer exists")
	})

	t.Run("success when file exists", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test.jpg")
		require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

		mock := &mockPlatform{wallpaperPath: testFile}
		e := &Engine{platform: mock}

		err := e.OpenInFinder()
		require.NoError(t, err)
		assert.True(t, mock.revealCalled)
	})
}

func TestEngine_OpenImage(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("error when platform returns empty path", func(t *testing.T) {
		mock := &mockPlatform{wallpaperPath: ""}
		e := &Engine{platform: mock}

		err := e.OpenImage()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no wallpaper path available")
	})

	t.Run("error when file does not exist", func(t *testing.T) {
		mock := &mockPlatform{wallpaperPath: "/nonexistent/wallpaper.jpg"}
		e := &Engine{platform: mock}

		err := e.OpenImage()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no longer exists")
	})

	t.Run("success when file exists", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test.jpg")
		require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

		mock := &mockPlatform{wallpaperPath: testFile}
		e := &Engine{platform: mock}

		err := e.OpenImage()
		require.NoError(t, err)
		assert.True(t, mock.openCalled)
	})
}

func TestEngine_pickFromProvider(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	// Create test directories
	localDir := filepath.Join(tmpDir, "local")
	uploadDir := filepath.Join(tmpDir, "upload")
	require.NoError(t, os.MkdirAll(localDir, 0755))
	require.NoError(t, os.MkdirAll(uploadDir, 0755))

	// Create a test image
	require.NoError(t, os.WriteFile(filepath.Join(localDir, "test.jpg"), []byte("test"), 0644))

	t.Run("local provider picks from local sources", func(t *testing.T) {
		st := state.New(statePath)
		cfg := &config.Config{
			Providers: map[string]config.ProviderConfig{
				"local": {Recursive: true},
			},
			Light: config.ThemeConfig{
				Dirs:      []string{localDir},
				UploadDir: uploadDir,
			},
		}

		manager := datasource.NewManager(uploadDir, tmpDir)
		manager.AddLocalSource(datasource.NewLocalSource("light-local-1", localDir, "light", false))

		e := &Engine{
			config:  cfg,
			state:   st,
			manager: manager,
		}

		img, isTemp, err := e.pickFromProvider(context.Background(), "light", "local")
		require.NoError(t, err)
		assert.False(t, isTemp)
		assert.NotNil(t, img)
		assert.Contains(t, img.Path, "test.jpg")
	})

	t.Run("local provider fails when no local sources", func(t *testing.T) {
		st := state.New(statePath)
		cfg := &config.Config{
			Providers: map[string]config.ProviderConfig{},
		}

		manager := datasource.NewManager(uploadDir, tmpDir)
		// No local sources added

		e := &Engine{
			config:  cfg,
			state:   st,
			manager: manager,
		}

		img, _, err := e.pickFromProvider(context.Background(), "light", "local")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to pick from local")
		assert.Nil(t, img)
	})

	t.Run("remote provider creates source on-the-fly when not in manager", func(t *testing.T) {
		st := state.New(statePath)
		cfg := &config.Config{
			Theme: config.ThemeSettings{Mode: config.ThemeModeDark},
			Providers: map[string]config.ProviderConfig{
				"local": {Recursive: true},
			},
			Dark: config.ThemeConfig{
				Dirs:      []string{localDir},
				UploadDir: uploadDir,
				Queries:   []string{"test"},
			},
		}

		manager := datasource.NewManager(uploadDir, tmpDir)
		// No remote sources added - bing should be created on-the-fly

		e := &Engine{
			config:        cfg,
			state:         st,
			manager:       manager,
			themeOverride: "dark",
		}

		// This should create bing provider on-the-fly and fetch
		// Note: This will actually call the real Bing API
		// In a real test we'd mock the HTTP client
		img, isTemp, err := e.pickFromProvider(context.Background(), "dark", "bing")

		// Bing API should work (it's public and free)
		if err != nil {
			// If network fails, check it's not a "not configured" error
			assert.NotContains(t, err.Error(), "not configured")
		} else {
			assert.True(t, isTemp)
			assert.NotNil(t, img)
			assert.Equal(t, "bing", img.SourceID[5:]) // "dark-bing"
		}
	})

	t.Run("uses existing remote source when available", func(t *testing.T) {
		st := state.New(statePath)
		cfg := &config.Config{
			Theme: config.ThemeSettings{Mode: config.ThemeModeDark},
			Providers: map[string]config.ProviderConfig{
				"bing": {},
			},
			Dark: config.ThemeConfig{
				Dirs:      []string{localDir},
				UploadDir: uploadDir,
			},
		}

		manager := datasource.NewManager(uploadDir, tmpDir)
		// Add bing source to manager
		manager.AddRemoteSource(datasource.NewRemoteSource("dark-bing", "bing", "", "dark", uploadDir, tmpDir, nil, 1, nil))

		e := &Engine{
			config:        cfg,
			state:         st,
			manager:       manager,
			themeOverride: "dark",
		}

		// Should use existing source from manager
		img, isTemp, err := e.pickFromProvider(context.Background(), "dark", "bing")

		if err != nil {
			// Network errors are OK in tests
			assert.NotContains(t, err.Error(), "not configured")
		} else {
			assert.True(t, isTemp)
			assert.NotNil(t, img)
		}
	})
}
