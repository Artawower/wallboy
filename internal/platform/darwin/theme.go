//go:build darwin

package darwin

import (
	"os/exec"
	"strings"

	"github.com/Artawower/wallboy/internal/platform"
)

type ThemeService struct{}

func NewThemeService() *ThemeService {
	return &ThemeService{}
}

func (s *ThemeService) Detect() platform.Theme {
	cmd := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle")
	output, err := cmd.Output()
	if err != nil {
		return platform.ThemeLight
	}

	style := strings.TrimSpace(string(output))
	if strings.EqualFold(style, "dark") {
		return platform.ThemeDark
	}

	return platform.ThemeLight
}
