// Package core provides the main business logic for wallboy.
package core

import (
	"time"

	"github.com/darkawower/wallboy/internal/platform"
)

// WallpaperResult represents the result of a wallpaper operation.
type WallpaperResult struct {
	// Path is the absolute path to the wallpaper image.
	Path string

	// Theme is the theme (light/dark) of the wallpaper.
	Theme string

	// SourceID is the identifier of the datasource.
	SourceID string

	// IsTemp indicates if the wallpaper is temporary (not saved).
	IsTemp bool

	// SetAt is when the wallpaper was set.
	SetAt time.Time
}

// WallpaperInfo contains detailed information about the current wallpaper.
type WallpaperInfo struct {
	// Path is the absolute path to the wallpaper image.
	Path string

	// Theme is the theme (light/dark) of the wallpaper.
	Theme string

	// SourceID is the identifier of the datasource.
	SourceID string

	// IsTemp indicates if the wallpaper is temporary (not saved).
	IsTemp bool

	// SetAt is when the wallpaper was set.
	SetAt time.Time

	// Exists indicates if the file still exists on disk.
	Exists bool
}

// SourceInfo contains information about a datasource.
type SourceInfo struct {
	// ID is the unique identifier.
	ID string

	// Theme is the theme this source belongs to.
	Theme string

	// Type is the datasource type (local/remote).
	Type string

	// Description is a human-readable description (path or provider).
	Description string
}

// AgentStatus represents the status of the background agent.
type AgentStatus struct {
	// Supported indicates if the agent is supported on this platform.
	Supported bool

	// Installed indicates if the agent is installed.
	Installed bool

	// Running indicates if the agent is currently running.
	Running bool

	// Interval is the time between wallpaper changes.
	Interval time.Duration

	// LogPath is the path to the agent log file.
	LogPath string
}

// Color represents a color extracted from an image.
type Color struct {
	R, G, B uint8
}

// Hex returns the hex representation of the color.
func (c Color) Hex() string {
	return "#" + hexByte(c.R) + hexByte(c.G) + hexByte(c.B)
}

func hexByte(b uint8) string {
	const hex = "0123456789abcdef"
	return string([]byte{hex[b>>4], hex[b&0x0f]})
}

// ThemeMode represents the theme detection mode.
type ThemeMode string

const (
	ThemeModeAuto  ThemeMode = "auto"
	ThemeModeLight ThemeMode = "light"
	ThemeModeDark  ThemeMode = "dark"
)

// Theme represents the current system theme.
type Theme string

const (
	ThemeLight Theme = "light"
	ThemeDark  Theme = "dark"
)

// ToPlatformTheme converts Theme to platform.Theme.
func (t Theme) ToPlatformTheme() platform.Theme {
	if t == ThemeDark {
		return platform.ThemeDark
	}
	return platform.ThemeLight
}

// FromPlatformTheme converts platform.Theme to Theme.
func FromPlatformTheme(t platform.Theme) Theme {
	if t == platform.ThemeDark {
		return ThemeDark
	}
	return ThemeLight
}
