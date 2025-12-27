//go:build darwin

package darwin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/darkawower/wallboy/internal/platform"
)

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>%s
    </array>
    <key>StartInterval</key>
    <integer>%d</integer>
    <key>RunAtLoad</key>
    <%s/>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
</dict>
</plist>
`

// SchedulerService implements platform.SchedulerService using launchctl.
type SchedulerService struct{}

// NewSchedulerService creates a new macOS scheduler service.
func NewSchedulerService() *SchedulerService {
	return &SchedulerService{}
}

// IsSupported returns true as launchctl is available on macOS.
func (s *SchedulerService) IsSupported() bool {
	return true
}

// Install creates and loads a launchd plist for the scheduled task.
func (s *SchedulerService) Install(config platform.SchedulerConfig) error {
	plistPath, err := s.getPlistPath(config.Label)
	if err != nil {
		return fmt.Errorf("failed to get plist path: %w", err)
	}

	// Ensure LaunchAgents directory exists
	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	// Ensure log directory exists
	if config.LogPath != "" {
		if err := os.MkdirAll(filepath.Dir(config.LogPath), 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}
	}

	// Unload existing agent if present (ignore errors)
	if _, err := os.Stat(plistPath); err == nil {
		_ = exec.Command("launchctl", "unload", plistPath).Run()
	}

	// Build args string
	argsStr := ""
	for _, arg := range config.Args {
		argsStr += fmt.Sprintf("\n        <string>%s</string>", arg)
	}

	// RunAtLoad value
	runAtLoad := "false"
	if config.RunAtLoad {
		runAtLoad = "true"
	}

	// Generate plist content
	intervalSeconds := int(config.Interval.Seconds())
	plistContent := fmt.Sprintf(plistTemplate,
		config.Label,
		config.Command,
		argsStr,
		intervalSeconds,
		runAtLoad,
		config.LogPath,
		config.LogPath,
	)

	// Write plist file
	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	// Load agent
	cmd := exec.Command("launchctl", "load", plistPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to load agent: %w (output: %s)", err, string(output))
	}

	return nil
}

// Uninstall unloads and removes the launchd plist.
func (s *SchedulerService) Uninstall(label string) error {
	plistPath, err := s.getPlistPath(label)
	if err != nil {
		return fmt.Errorf("failed to get plist path: %w", err)
	}

	// Check if installed
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return nil // Not installed, nothing to do
	}

	// Unload agent
	cmd := exec.Command("launchctl", "unload", plistPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Log warning but continue to remove file
		_ = output // Ignore output for now
	}

	// Remove plist file
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist: %w", err)
	}

	return nil
}

// Status returns the current status of the scheduled task.
func (s *SchedulerService) Status(label string) (platform.SchedulerStatus, error) {
	plistPath, err := s.getPlistPath(label)
	if err != nil {
		return platform.SchedulerStatus{}, fmt.Errorf("failed to get plist path: %w", err)
	}

	status := platform.SchedulerStatus{}

	// Check if plist exists
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return status, nil // Not installed
	}
	status.Installed = true

	// Check if running via launchctl list
	cmd := exec.Command("launchctl", "list", label)
	status.Running = cmd.Run() == nil

	// Read interval from plist
	interval, logPath, err := s.parsePlist(plistPath)
	if err == nil {
		status.Interval = interval
		status.LogPath = logPath
	}

	return status, nil
}

// getPlistPath returns the path to the launchd plist file.
func (s *SchedulerService) getPlistPath(label string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist"), nil
}

// parsePlist extracts interval and log path from a plist file.
func (s *SchedulerService) parsePlist(plistPath string) (time.Duration, string, error) {
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return 0, "", err
	}

	content := string(data)

	// Extract interval
	intervalRe := regexp.MustCompile(`<key>StartInterval</key>\s*<integer>(\d+)</integer>`)
	intervalMatches := intervalRe.FindStringSubmatch(content)
	var interval time.Duration
	if len(intervalMatches) >= 2 {
		seconds, _ := strconv.Atoi(intervalMatches[1])
		interval = time.Duration(seconds) * time.Second
	}

	// Extract log path
	logRe := regexp.MustCompile(`<key>StandardOutPath</key>\s*<string>([^<]+)</string>`)
	logMatches := logRe.FindStringSubmatch(content)
	var logPath string
	if len(logMatches) >= 2 {
		logPath = logMatches[1]
	}

	return interval, logPath, nil
}
