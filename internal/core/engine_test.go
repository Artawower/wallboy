package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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

	WithSourceOverride("wallhaven")(e)
	assert.Equal(t, "wallhaven", e.sourceOverride)

	WithDryRun(true)(e)
	assert.True(t, e.dryRun)
}
