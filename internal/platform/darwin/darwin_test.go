//go:build darwin

package darwin

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Artawower/wallboy/internal/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlatformInterface(t *testing.T) {
	p := New()

	// Verify it implements Platform interface
	var _ platform.Platform = p

	assert.Equal(t, "darwin", p.Name())
	assert.True(t, p.IsSupported())
	assert.NotNil(t, p.Wallpaper())
	assert.NotNil(t, p.Theme())
	assert.NotNil(t, p.Scheduler())
	assert.NotNil(t, p.FileManager())
}

func TestThemeService(t *testing.T) {
	svc := NewThemeService()
	theme := svc.Detect()

	// Should return a valid theme
	assert.True(t, theme == platform.ThemeLight || theme == platform.ThemeDark)
}

func TestSchedulerService_IsSupported(t *testing.T) {
	svc := NewSchedulerService()
	assert.True(t, svc.IsSupported())
}

func TestSchedulerService_GetPlistPath(t *testing.T) {
	svc := NewSchedulerService()
	path, err := svc.getPlistPath("com.test.agent")

	require.NoError(t, err)
	assert.Contains(t, path, "Library/LaunchAgents")
	assert.Contains(t, path, "com.test.agent.plist")
}

func TestSchedulerService_ParsePlist(t *testing.T) {
	svc := NewSchedulerService()

	// Create temp plist
	tmpDir := t.TempDir()
	plistPath := filepath.Join(tmpDir, "test.plist")

	plistContent := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.test.agent</string>
    <key>StartInterval</key>
    <integer>300</integer>
    <key>StandardOutPath</key>
    <string>/tmp/test.log</string>
</dict>
</plist>
`
	err := os.WriteFile(plistPath, []byte(plistContent), 0644)
	require.NoError(t, err)

	interval, logPath, err := svc.parsePlist(plistPath)

	require.NoError(t, err)
	assert.Equal(t, 5*time.Minute, interval)
	assert.Equal(t, "/tmp/test.log", logPath)
}

func TestSchedulerService_StatusNotInstalled(t *testing.T) {
	svc := NewSchedulerService()

	// Use a label that definitely doesn't exist
	status, err := svc.Status("com.wallboy.test.nonexistent.12345")

	require.NoError(t, err)
	assert.False(t, status.Installed)
	assert.False(t, status.Running)
}

func TestWallpaperService(t *testing.T) {
	svc := NewWallpaperService()

	// We can't test Set without changing the actual wallpaper
	// But we can test Get
	path, err := svc.Get()

	// Should not error (may return empty if no wallpaper set)
	if err == nil {
		t.Logf("Current wallpaper: %s", path)
	}
}

func TestFileManagerService(t *testing.T) {
	svc := NewFileManagerService()

	// We can't really test Reveal/Open without side effects
	// Just verify the service is created
	assert.NotNil(t, svc)
}
