//go:build darwin

package darwin

import (
	"fmt"
	"os/exec"
	"strings"
)

type WallpaperService struct{}

func NewWallpaperService() *WallpaperService {
	return &WallpaperService{}
}

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
