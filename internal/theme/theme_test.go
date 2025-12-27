package theme

import (
	"testing"

	"github.com/Artawower/wallboy/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestNewDetector(t *testing.T) {
	d := NewDetector(config.ThemeModeAuto)
	assert.NotNil(t, d)
	assert.Equal(t, config.ThemeModeAuto, d.mode)
}

func TestDetector_Detect(t *testing.T) {
	tests := []struct {
		name     string
		mode     config.ThemeMode
		expected Theme
	}{
		{
			name:     "light mode",
			mode:     config.ThemeModeLight,
			expected: Light,
		},
		{
			name:     "dark mode",
			mode:     config.ThemeModeDark,
			expected: Dark,
		},
		{
			name:     "unknown mode defaults to light",
			mode:     config.ThemeMode("unknown"),
			expected: Light,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDetector(tt.mode)
			result := d.Detect()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetector_Detect_Auto(t *testing.T) {
	// Auto mode will call system detection
	// We can't predict the result, but we can verify it returns a valid theme
	d := NewDetector(config.ThemeModeAuto)
	result := d.Detect()

	// Should be either Light or Dark
	assert.True(t, result == Light || result == Dark)
}

func TestTheme_ToConfigMode(t *testing.T) {
	tests := []struct {
		name     string
		theme    Theme
		expected config.ThemeMode
	}{
		{
			name:     "light",
			theme:    Light,
			expected: config.ThemeModeLight,
		},
		{
			name:     "dark",
			theme:    Dark,
			expected: config.ThemeModeDark,
		},
		{
			name:     "unknown defaults to light",
			theme:    Theme("unknown"),
			expected: config.ThemeModeLight,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.theme.ToConfigMode()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTheme_String(t *testing.T) {
	assert.Equal(t, "light", Light.String())
	assert.Equal(t, "dark", Dark.String())
}

func TestThemeConstants(t *testing.T) {
	assert.Equal(t, Theme("light"), Light)
	assert.Equal(t, Theme("dark"), Dark)
}

// Test detectSystem - this tests the branch logic using platform abstraction
func TestDetector_detectSystem(t *testing.T) {
	d := NewDetector(config.ThemeModeAuto)

	// This will call the platform-specific detection via platform.ThemeService
	// We can't control the output, but we verify it doesn't panic
	result := d.detectSystem()
	assert.True(t, result == Light || result == Dark)
}
