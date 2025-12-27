//go:build darwin

package darwin

import (
	"fmt"
	"os/exec"
	"strings"
)

// WallpaperService implements platform.WallpaperService for macOS.
type WallpaperService struct{}

// NewWallpaperService creates a new macOS wallpaper service.
func NewWallpaperService() *WallpaperService {
	return &WallpaperService{}
}

// Set sets the desktop wallpaper using AppleScript.
func (s *WallpaperService) Set(path string) error {
	script := fmt.Sprintf(`tell application "System Events"
		tell every desktop
			set picture to "%s"
		end tell
	end tell`, path)

	cmd := exec.Command("osascript", "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set wallpaper: %w (output: %s)", err, string(output))
	}
	return nil
}

// Get returns the current desktop wallpaper path.
// Uses Finder which is more reliable across multiple desktops/displays.
func (s *WallpaperService) Get() (string, error) {
	script := `tell application "Finder" to get POSIX path of (desktop picture as alias)`

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get wallpaper: %w", err)
	}

	path := strings.TrimSpace(string(output))
	return path, nil
}
