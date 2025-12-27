// Package darwin provides macOS-specific platform implementations.
package darwin

import "github.com/darkawower/wallboy/internal/platform"

func init() {
	platform.Register("darwin", func() platform.Platform {
		return New()
	})
}

// Platform implements platform.Platform for macOS.
type Platform struct {
	wallpaper   *WallpaperService
	theme       *ThemeService
	scheduler   *SchedulerService
	fileManager *FileManagerService
}

// New creates a new macOS platform instance.
func New() *Platform {
	return &Platform{
		wallpaper:   NewWallpaperService(),
		theme:       NewThemeService(),
		scheduler:   NewSchedulerService(),
		fileManager: NewFileManagerService(),
	}
}

// Name returns the platform identifier.
func (p *Platform) Name() string {
	return "darwin"
}

// IsSupported returns true as macOS is fully supported.
func (p *Platform) IsSupported() bool {
	return true
}

// Wallpaper returns the wallpaper management service.
func (p *Platform) Wallpaper() platform.WallpaperService {
	return p.wallpaper
}

// Theme returns the theme detection service.
func (p *Platform) Theme() platform.ThemeService {
	return p.theme
}

// Scheduler returns the background task scheduler service.
func (p *Platform) Scheduler() platform.SchedulerService {
	return p.scheduler
}

// FileManager returns the file manager service.
func (p *Platform) FileManager() platform.FileManagerService {
	return p.fileManager
}

// Compile-time check that Platform implements platform.Platform.
var _ platform.Platform = (*Platform)(nil)
