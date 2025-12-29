//go:build darwin

package darwin

import "github.com/Artawower/wallboy/internal/platform"

func init() {
	platform.Register("darwin", func() platform.Platform {
		return New()
	})
}

type Platform struct {
	wallpaper   *WallpaperService
	theme       *ThemeService
	scheduler   *SchedulerService
	fileManager *FileManagerService
}

func New() *Platform {
	return &Platform{
		wallpaper:   NewWallpaperService(),
		theme:       NewThemeService(),
		scheduler:   NewSchedulerService(),
		fileManager: NewFileManagerService(),
	}
}

func (p *Platform) Name() string                             { return "darwin" }
func (p *Platform) IsSupported() bool                        { return true }
func (p *Platform) Wallpaper() platform.WallpaperService     { return p.wallpaper }
func (p *Platform) Theme() platform.ThemeService             { return p.theme }
func (p *Platform) Scheduler() platform.SchedulerService     { return p.scheduler }
func (p *Platform) FileManager() platform.FileManagerService { return p.fileManager }

var _ platform.Platform = (*Platform)(nil)
