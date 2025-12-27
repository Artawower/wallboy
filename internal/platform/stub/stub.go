// Package stub provides a fallback platform implementation for unsupported systems.
package stub

import (
	"fmt"
	"runtime"

	"github.com/darkawower/wallboy/internal/platform"
)

func init() {
	// Register stub as fallback for unsupported platforms
	// This will be overridden if a specific platform registers itself
	for _, os := range []string{"freebsd", "openbsd", "netbsd", "dragonfly", "solaris", "aix"} {
		platform.Register(os, func() platform.Platform {
			return New()
		})
	}
}

// Platform implements platform.Platform as a fallback for unsupported systems.
type Platform struct {
	name string
}

// New creates a new stub platform instance.
func New() *Platform {
	return &Platform{
		name: runtime.GOOS,
	}
}

// Name returns the platform identifier.
func (p *Platform) Name() string {
	return p.name
}

// IsSupported returns false as this is a fallback implementation.
func (p *Platform) IsSupported() bool {
	return false
}

// Wallpaper returns the wallpaper service (stub).
func (p *Platform) Wallpaper() platform.WallpaperService {
	return &stubWallpaperService{}
}

// Theme returns the theme detection service (stub).
func (p *Platform) Theme() platform.ThemeService {
	return &stubThemeService{}
}

// Scheduler returns the scheduler service (stub).
func (p *Platform) Scheduler() platform.SchedulerService {
	return &stubSchedulerService{}
}

// FileManager returns the file manager service (stub).
func (p *Platform) FileManager() platform.FileManagerService {
	return &stubFileManagerService{}
}

// Compile-time check that Platform implements platform.Platform.
var _ platform.Platform = (*Platform)(nil)

// stubWallpaperService is a no-op wallpaper service.
type stubWallpaperService struct{}

func (s *stubWallpaperService) Set(path string) error {
	return fmt.Errorf("wallpaper setting not supported on %s", runtime.GOOS)
}

func (s *stubWallpaperService) Get() (string, error) {
	return "", fmt.Errorf("wallpaper detection not supported on %s", runtime.GOOS)
}

// stubThemeService always returns light theme.
type stubThemeService struct{}

func (s *stubThemeService) Detect() platform.Theme {
	return platform.ThemeLight
}

// stubSchedulerService is a no-op scheduler service.
type stubSchedulerService struct{}

func (s *stubSchedulerService) Install(config platform.SchedulerConfig) error {
	return fmt.Errorf("scheduler not supported on %s", runtime.GOOS)
}

func (s *stubSchedulerService) Uninstall(label string) error {
	return fmt.Errorf("scheduler not supported on %s", runtime.GOOS)
}

func (s *stubSchedulerService) Status(label string) (platform.SchedulerStatus, error) {
	return platform.SchedulerStatus{}, fmt.Errorf("scheduler not supported on %s", runtime.GOOS)
}

func (s *stubSchedulerService) IsSupported() bool {
	return false
}

// stubFileManagerService is a no-op file manager service.
type stubFileManagerService struct{}

func (s *stubFileManagerService) Reveal(path string) error {
	return fmt.Errorf("file manager not supported on %s", runtime.GOOS)
}

func (s *stubFileManagerService) Open(path string) error {
	return fmt.Errorf("file manager not supported on %s", runtime.GOOS)
}
