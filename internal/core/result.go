package core

import (
	"time"

	"github.com/Artawower/wallboy/internal/platform"
)

type WallpaperResult struct {
	Path     string
	Theme    string
	SourceID string
	IsTemp   bool
	SetAt    time.Time
	Query    string
}

type WallpaperInfo struct {
	Path     string
	Theme    string
	SourceID string
	IsTemp   bool
	SetAt    time.Time
	Exists   bool
	Query    string
}

type SourceInfo struct {
	ID          string
	Theme       string
	Type        string
	Description string
}

type AgentStatus struct {
	Supported bool
	Installed bool
	Running   bool
	Interval  time.Duration
	LogPath   string
}

type Color struct {
	R, G, B uint8
}

func (c Color) Hex() string {
	return "#" + hexByte(c.R) + hexByte(c.G) + hexByte(c.B)
}

func hexByte(b uint8) string {
	const hex = "0123456789abcdef"
	return string([]byte{hex[b>>4], hex[b&0x0f]})
}

type ThemeMode string

const (
	ThemeModeAuto  ThemeMode = "auto"
	ThemeModeLight ThemeMode = "light"
	ThemeModeDark  ThemeMode = "dark"
)

type Theme string

const (
	ThemeLight Theme = "light"
	ThemeDark  Theme = "dark"
)

func (t Theme) ToPlatformTheme() platform.Theme {
	if t == ThemeDark {
		return platform.ThemeDark
	}
	return platform.ThemeLight
}

func FromPlatformTheme(t platform.Theme) Theme {
	if t == platform.ThemeDark {
		return ThemeDark
	}
	return ThemeLight
}
