package platform

import (
	"errors"
	"runtime"
	"sync"
)

var ErrUnsupported = errors.New("operation not supported on this platform")

type platformBuilder func() Platform

var (
	registry     = make(map[string]platformBuilder)
	registryLock sync.RWMutex
)

func Register(osName string, builder platformBuilder) {
	registryLock.Lock()
	defer registryLock.Unlock()
	registry[osName] = builder
}

var (
	current     Platform
	currentOnce sync.Once
)

func Current() Platform {
	currentOnce.Do(func() {
		current = newPlatform()
	})
	return current
}

func newPlatform() Platform {
	registryLock.RLock()
	defer registryLock.RUnlock()

	if builder, ok := registry[runtime.GOOS]; ok {
		return builder()
	}

	return &unsupportedPlatform{name: runtime.GOOS}
}

type unsupportedPlatform struct {
	name string
}

func (p *unsupportedPlatform) Name() string                    { return p.name }
func (p *unsupportedPlatform) IsSupported() bool               { return false }
func (p *unsupportedPlatform) Wallpaper() WallpaperService     { return &unsupportedWallpaper{} }
func (p *unsupportedPlatform) Theme() ThemeService             { return &unsupportedTheme{} }
func (p *unsupportedPlatform) Scheduler() SchedulerService     { return &unsupportedScheduler{} }
func (p *unsupportedPlatform) FileManager() FileManagerService { return &unsupportedFileManager{} }

type unsupportedWallpaper struct{}

func (s *unsupportedWallpaper) Set(path string) error { return ErrUnsupported }
func (s *unsupportedWallpaper) Get() (string, error)  { return "", ErrUnsupported }

type unsupportedTheme struct{}

func (s *unsupportedTheme) Detect() Theme { return ThemeLight }

type unsupportedScheduler struct{}

func (s *unsupportedScheduler) Install(config SchedulerConfig) error { return ErrUnsupported }
func (s *unsupportedScheduler) Uninstall(label string) error         { return ErrUnsupported }
func (s *unsupportedScheduler) Status(label string) (SchedulerStatus, error) {
	return SchedulerStatus{}, ErrUnsupported
}
func (s *unsupportedScheduler) IsSupported() bool { return false }

type unsupportedFileManager struct{}

func (s *unsupportedFileManager) Reveal(path string) error { return ErrUnsupported }
func (s *unsupportedFileManager) Open(path string) error   { return ErrUnsupported }

func SetPlatform(p Platform) {
	currentOnce.Do(func() {})
	current = p
}

func ResetPlatform() {
	currentOnce = sync.Once{}
	current = nil
}
