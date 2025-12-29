package theme

import (
	"github.com/Artawower/wallboy/internal/config"
	"github.com/Artawower/wallboy/internal/platform"
)

type Theme string

const (
	Light Theme = "light"
	Dark  Theme = "dark"
)

type Detector struct {
	mode config.ThemeMode
	svc  platform.ThemeService
}

func NewDetector(mode config.ThemeMode) *Detector {
	return &Detector{
		mode: mode,
		svc:  platform.Current().Theme(),
	}
}

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

func (d *Detector) detectSystem() Theme {
	platformTheme := d.svc.Detect()
	switch platformTheme {
	case platform.ThemeDark:
		return Dark
	default:
		return Light
	}
}

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

func (t Theme) String() string {
	return string(t)
}
