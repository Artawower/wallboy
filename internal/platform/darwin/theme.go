package darwin

import (
	"os/exec"
	"strings"

	"github.com/darkawower/wallboy/internal/platform"
)

// ThemeService implements platform.ThemeService for macOS.
type ThemeService struct{}

// NewThemeService creates a new macOS theme service.
func NewThemeService() *ThemeService {
	return &ThemeService{}
}

// Detect returns the current system theme by reading AppleInterfaceStyle.
func (s *ThemeService) Detect() platform.Theme {
	cmd := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle")
	output, err := cmd.Output()
	if err != nil {
		// Command fails or returns empty when in light mode
		return platform.ThemeLight
	}

	style := strings.TrimSpace(string(output))
	if strings.EqualFold(style, "dark") {
		return platform.ThemeDark
	}

	return platform.ThemeLight
}
