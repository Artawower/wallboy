// Package theme handles theme detection for wallboy.
package theme

import (
	"github.com/darkawower/wallboy/internal/config"
	"github.com/darkawower/wallboy/internal/platform"
)

// Theme represents the current theme.
type Theme string

const (
	Light Theme = "light"
	Dark  Theme = "dark"
)

// Detector detects the current system theme.
type Detector struct {
	mode config.ThemeMode
	svc  platform.ThemeService
}

// NewDetector creates a new theme detector.
func NewDetector(mode config.ThemeMode) *Detector {
	return &Detector{
		mode: mode,
		svc:  platform.Current().Theme(),
	}
}

// Detect detects the current theme based on configuration and system settings.
func (d *Detector) Detect() Theme {
	switch d.mode {
	case config.ThemeModeLight:
		return Light
	case config.ThemeModeDark:
		return Dark
	case config.ThemeModeAuto:
		return d.detectSystem()
	default:
		return Light
	}
}

// detectSystem detects the system theme using the platform service.
func (d *Detector) detectSystem() Theme {
	platformTheme := d.svc.Detect()
	switch platformTheme {
	case platform.ThemeDark:
		return Dark
	default:
		return Light
	}
}

// ToConfigMode converts Theme to config.ThemeMode.
func (t Theme) ToConfigMode() config.ThemeMode {
	switch t {
	case Light:
		return config.ThemeModeLight
	case Dark:
		return config.ThemeModeDark
	default:
		return config.ThemeModeLight
	}
}

// String returns the string representation of the theme.
func (t Theme) String() string {
	return string(t)
}
