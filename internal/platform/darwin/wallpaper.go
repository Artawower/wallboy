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
func (s *WallpaperService) Get() (string, error) {
	script := `tell application "System Events" to get picture of first desktop`

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get wallpaper: %w", err)
	}

	path := strings.TrimSpace(string(output))
	return path, nil
}
