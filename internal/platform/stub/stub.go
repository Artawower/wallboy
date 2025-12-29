package stub

import (
	"fmt"
	"runtime"

	"github.com/Artawower/wallboy/internal/platform"
)

func init() {
	for _, os := range []string{"freebsd", "openbsd", "netbsd", "dragonfly", "solaris", "aix"} {
		platform.Register(os, func() platform.Platform {
			return New()
		})
	}
}

type Platform struct {
	name string
}

func New() *Platform {
	return &Platform{
		name: runtime.GOOS,
	}
}

func (p *Platform) Name() string                             { return p.name }
func (p *Platform) IsSupported() bool                        { return false }
func (p *Platform) Wallpaper() platform.WallpaperService     { return &stubWallpaperService{} }
func (p *Platform) Theme() platform.ThemeService             { return &stubThemeService{} }
func (p *Platform) Scheduler() platform.SchedulerService     { return &stubSchedulerService{} }
func (p *Platform) FileManager() platform.FileManagerService { return &stubFileManagerService{} }

var _ platform.Platform = (*Platform)(nil)

type stubWallpaperService struct{}

func (s *stubWallpaperService) Set(path string) error {
	return fmt.Errorf("wallpaper setting not supported on %s", runtime.GOOS)
}

func (s *stubWallpaperService) Get() (string, error) {
	return "", fmt.Errorf("wallpaper detection not supported on %s", runtime.GOOS)
}

type stubThemeService struct{}

func (s *stubThemeService) Detect() platform.Theme {
	return platform.ThemeLight
}

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

type stubFileManagerService struct{}

func (s *stubFileManagerService) Reveal(path string) error {
	return fmt.Errorf("file manager not supported on %s", runtime.GOOS)
}

func (s *stubFileManagerService) Open(path string) error {
	return fmt.Errorf("file manager not supported on %s", runtime.GOOS)
}
