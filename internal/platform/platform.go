// Package platform provides OS-agnostic abstractions for system operations.
package platform

import "time"

// Theme represents the system color theme.
type Theme string

const (
	ThemeLight Theme = "light"
	ThemeDark  Theme = "dark"
)

// Platform provides access to OS-specific services.
type Platform interface {
	// Name returns the platform identifier (e.g., "darwin", "linux", "windows").
	Name() string

	// IsSupported returns true if this platform is fully supported.
	IsSupported() bool

	// Wallpaper returns the wallpaper management service.
	Wallpaper() WallpaperService

	// Theme returns the theme detection service.
	Theme() ThemeService

	// Scheduler returns the background task scheduler service.
	Scheduler() SchedulerService

	// FileManager returns the file manager service.
	FileManager() FileManagerService
}

// WallpaperService manages desktop wallpaper.
type WallpaperService interface {
	// Set sets the desktop wallpaper to the specified image path.
	Set(path string) error

	// Get returns the current desktop wallpaper path.
	Get() (string, error)
}

// ThemeService detects system color theme.
type ThemeService interface {
	// Detect returns the current system theme (light or dark).
	Detect() Theme
}

// SchedulerService manages background task scheduling.
type SchedulerService interface {
	// Install installs a scheduled task with the given configuration.
	Install(config SchedulerConfig) error

	// Uninstall removes the scheduled task by label.
	Uninstall(label string) error

	// Status returns the current status of the scheduled task by label.
	Status(label string) (SchedulerStatus, error)

	// IsSupported returns true if scheduling is supported on this platform.
	IsSupported() bool
}

// SchedulerConfig holds configuration for a scheduled task.
type SchedulerConfig struct {
	// Label is the unique identifier for the task.
	Label string

	// Command is the executable path.
	Command string

	// Args are the command arguments.
	Args []string

	// Interval is the time between executions.
	Interval time.Duration

	// RunAtLoad indicates whether to run immediately when loaded.
	RunAtLoad bool

	// LogPath is the path for stdout/stderr output.
	LogPath string
}

// SchedulerStatus represents the current state of a scheduled task.
type SchedulerStatus struct {
	// Installed indicates whether the task is installed.
	Installed bool

	// Running indicates whether the task is currently active.
	Running bool

	// Interval is the configured interval between executions.
	Interval time.Duration

	// LogPath is the configured log file path.
	LogPath string
}

// FileManagerService provides file manager operations.
type FileManagerService interface {
	// Reveal opens the file manager and highlights the specified path.
	Reveal(path string) error

	// Open opens the file with the default application.
	Open(path string) error
}
