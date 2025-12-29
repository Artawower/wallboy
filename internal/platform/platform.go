package platform

import "time"

type Theme string

const (
	ThemeLight Theme = "light"
	ThemeDark  Theme = "dark"
)

type Platform interface {
	Name() string
	IsSupported() bool
	Wallpaper() WallpaperService
	Theme() ThemeService
	Scheduler() SchedulerService
	FileManager() FileManagerService
}

type WallpaperService interface {
	Set(path string) error
	Get() (string, error)
}

type ThemeService interface {
	Detect() Theme
}

type SchedulerService interface {
	Install(config SchedulerConfig) error
	Uninstall(label string) error
	Status(label string) (SchedulerStatus, error)
	IsSupported() bool
}

type SchedulerConfig struct {
	Label     string
	Command   string
	Args      []string
	Interval  time.Duration
	RunAtLoad bool
	LogPath   string
}

type SchedulerStatus struct {
	Installed bool
	Running   bool
	Interval  time.Duration
	LogPath   string
}

type FileManagerService interface {
	Reveal(path string) error
	Open(path string) error
}
