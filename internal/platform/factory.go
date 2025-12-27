package platform

import (
	"errors"
	"runtime"
	"sync"
)

// ErrUnsupported is returned when an operation is not supported on the current platform.
var ErrUnsupported = errors.New("operation not supported on this platform")

// platformBuilder is a function that creates a Platform instance.
type platformBuilder func() Platform

// Registry holds registered platform builders.
var (
	registry     = make(map[string]platformBuilder)
	registryLock sync.RWMutex
)

// Register registers a platform builder for the given OS name.
// This should be called from init() functions in platform-specific packages.
func Register(osName string, builder platformBuilder) {
	registryLock.Lock()
	defer registryLock.Unlock()
	registry[osName] = builder
}

// current holds the cached platform instance.
var (
	current     Platform
	currentOnce sync.Once
)

// Current returns the platform implementation for the current OS.
// The instance is cached and reused for subsequent calls.
func Current() Platform {
	currentOnce.Do(func() {
		current = newPlatform()
	})
	return current
}

// newPlatform creates a new platform instance for the current OS.
func newPlatform() Platform {
	registryLock.RLock()
	defer registryLock.RUnlock()

	if builder, ok := registry[runtime.GOOS]; ok {
		return builder()
	}

	// Return an unsupported platform stub
	return &unsupportedPlatform{name: runtime.GOOS}
}

// unsupportedPlatform is a minimal fallback for unregistered platforms.
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

// SetPlatform allows overriding the current platform (useful for testing).
func SetPlatform(p Platform) {
	currentOnce.Do(func() {}) // Ensure once is triggered
	current = p
}

// ResetPlatform resets the cached platform (useful for testing).
func ResetPlatform() {
	currentOnce = sync.Once{}
	current = nil
}
