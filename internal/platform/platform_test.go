package platform

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestThemeConstants(t *testing.T) {
	assert.Equal(t, Theme("light"), ThemeLight)
	assert.Equal(t, Theme("dark"), ThemeDark)
}

func TestSchedulerConfig(t *testing.T) {
	config := SchedulerConfig{
		Label:     "com.test.agent",
		Command:   "/usr/local/bin/test",
		Args:      []string{"--flag", "value"},
		Interval:  10 * time.Minute,
		RunAtLoad: true,
		LogPath:   "/var/log/test.log",
	}

	assert.Equal(t, "com.test.agent", config.Label)
	assert.Equal(t, "/usr/local/bin/test", config.Command)
	assert.Equal(t, []string{"--flag", "value"}, config.Args)
	assert.Equal(t, 10*time.Minute, config.Interval)
	assert.True(t, config.RunAtLoad)
	assert.Equal(t, "/var/log/test.log", config.LogPath)
}

func TestSchedulerStatus(t *testing.T) {
	status := SchedulerStatus{
		Installed: true,
		Running:   true,
		Interval:  5 * time.Minute,
		LogPath:   "/var/log/test.log",
	}

	assert.True(t, status.Installed)
	assert.True(t, status.Running)
	assert.Equal(t, 5*time.Minute, status.Interval)
	assert.Equal(t, "/var/log/test.log", status.LogPath)
}

func TestErrUnsupported(t *testing.T) {
	assert.NotNil(t, ErrUnsupported)
	assert.Contains(t, ErrUnsupported.Error(), "not supported")
}
