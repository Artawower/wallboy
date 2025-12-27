package stub

import (
	"testing"

	"github.com/darkawower/wallboy/internal/platform"
	"github.com/stretchr/testify/assert"
)

func TestStubPlatform(t *testing.T) {
	p := New()

	// Verify it implements Platform interface
	var _ platform.Platform = p

	assert.NotEmpty(t, p.Name())
	assert.False(t, p.IsSupported())
}

func TestStubWallpaperService(t *testing.T) {
	p := New()
	svc := p.Wallpaper()

	err := svc.Set("/path/to/image")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")

	_, err = svc.Get()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestStubThemeService(t *testing.T) {
	p := New()
	svc := p.Theme()

	theme := svc.Detect()
	assert.Equal(t, platform.ThemeLight, theme)
}

func TestStubSchedulerService(t *testing.T) {
	p := New()
	svc := p.Scheduler()

	assert.False(t, svc.IsSupported())

	err := svc.Install(platform.SchedulerConfig{})
	assert.Error(t, err)

	err = svc.Uninstall("test")
	assert.Error(t, err)

	_, err = svc.Status("test")
	assert.Error(t, err)
}

func TestStubFileManagerService(t *testing.T) {
	p := New()
	svc := p.FileManager()

	err := svc.Reveal("/path")
	assert.Error(t, err)

	err = svc.Open("/path")
	assert.Error(t, err)
}
