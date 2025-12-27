// Package main is the entry point for the wallboy CLI.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/darkawower/wallboy/internal/config"
	"github.com/darkawower/wallboy/internal/core"
	"github.com/darkawower/wallboy/internal/state"
	"github.com/darkawower/wallboy/internal/ui"
	"github.com/spf13/cobra"
)

// Agent constants
const (
	defaultInterval = 600 // 10 minutes
	minInterval     = 60  // 1 minute minimum
)

var (
	// Global flags
	cfgFile    string
	themeFlag  string
	sourceFlag string
	dryRun     bool
	verbose    bool
	quiet      bool

	// Global output
	out *ui.Output
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "wallboy",
		Short: "Wallpaper manager for macOS",
		Long: `Wallboy is a CLI utility for managing desktop wallpapers on macOS.
It supports local and remote image sources, respects light/dark theme,
and provides additional features like color analysis.`,
	}

	// Persistent flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.config/wallboy/config.toml)")
	rootCmd.PersistentFlags().StringVar(&themeFlag, "theme", "", "theme to use (auto|light|dark)")
	rootCmd.PersistentFlags().StringVar(&sourceFlag, "source", "", "specific datasource to use")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "show what would be done without doing it")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress non-error output")

	// Add commands
	rootCmd.AddCommand(
		newInitCmd(),
		newNextCmd(),
		newSaveCmd(),
		newShowCmd(),
		newOpenCmd(),
		newInfoCmd(),
		newColorsCmd(),
		newDeleteCmd(),
		newSourcesCmd(),
		newVersionCmd(),
		newAgentInstallCmd(),
		newAgentUninstallCmd(),
		newAgentStatusCmd(),
	)

	// Handle signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

// initOutput initializes the output.
func initOutput() {
	out = ui.DefaultOutput()
	out.SetVerbose(verbose)
	out.SetQuiet(quiet)
}

// newEngine creates a new engine with current flags.
func newEngine() (*core.Engine, error) {
	var opts []core.Option
	if themeFlag != "" {
		opts = append(opts, core.WithThemeOverride(themeFlag))
	}
	if sourceFlag != "" {
		opts = append(opts, core.WithSourceOverride(sourceFlag))
	}
	if dryRun {
		opts = append(opts, core.WithDryRun(true))
	}

	return core.New(cfgFile, opts...)
}

// newInitCmd creates the init command.
func newInitCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize wallboy configuration",
		Long:  "Creates default configuration file and directories.",
		RunE: func(cmd *cobra.Command, args []string) error {
			initOutput()

			configDir := config.DefaultConfigDir()
			configPath := filepath.Join(configDir, "config.toml")

			// Check if already exists
			if _, err := os.Stat(configPath); err == nil && !force {
				out.Warning("Configuration already exists at %s", configPath)
				out.Info("Use --force to overwrite")
				return nil
			}

			// Create default config
			cfg := config.DefaultConfig()

			// Create directories
			if err := cfg.EnsureDirectories(); err != nil {
				out.Error("Failed to create directories: %v", err)
				return err
			}

			// Write config
			if err := cfg.Save(configPath); err != nil {
				out.Error("Failed to write config: %v", err)
				return err
			}

			// Create empty state file
			st := state.New(cfg.State.Path)
			if err := st.Save(); err != nil {
				out.Error("Failed to create state file: %v", err)
				return err
			}

			out.Success("Wallboy initialized")
			out.Field("Config", configPath)
			out.Field("State", cfg.State.Path)
			out.Field("Saved", cfg.UploadDir)
			out.Field("Temp", config.GetTempDir())
			out.Print("")
			out.Info("Edit %s to configure datasources", configPath)

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing configuration")

	return cmd
}

// newNextCmd creates the next command.
func newNextCmd() *cobra.Command {
	var openAfter bool

	cmd := &cobra.Command{
		Use:   "next",
		Short: "Set next random wallpaper",
		Long: `Selects and sets a random wallpaper from configured sources.

For remote sources: downloads image to temp directory.
Use 'wallboy save' to keep the image permanently.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			initOutput()

			engine, err := newEngine()
			if err != nil {
				out.ErrorWithHint(err.Error(), "Run 'wallboy init' to create a default configuration")
				return err
			}

			result, err := engine.Next(cmd.Context())
			if err != nil {
				out.Error("Failed to set wallpaper: %v", err)
				return err
			}

			if dryRun {
				out.Info("Would set wallpaper to: %s", result.Path)
				if result.IsTemp {
					out.Info("(temporary - use 'wallboy save' to keep)")
				}
				return nil
			}

			out.WallpaperInfo(result.Theme, result.SourceID, shortenPath(result.Path), result.SetAt)

			if result.IsTemp {
				out.Print("")
				out.Info("Use 'wallboy save' to keep this wallpaper")
			}

			if openAfter {
				if err := engine.OpenInFinder(); err != nil {
					out.Warning("Failed to open in Finder: %v", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&openAfter, "open", false, "open image in Finder after setting")

	return cmd
}

// newSaveCmd creates the save command.
func newSaveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "save",
		Short: "Save current wallpaper permanently",
		Long:  "Moves current wallpaper from temp to saved directory.",
		RunE: func(cmd *cobra.Command, args []string) error {
			initOutput()

			engine, err := newEngine()
			if err != nil {
				out.ErrorWithHint(err.Error(), "Run 'wallboy init' to create a default configuration")
				return err
			}

			result, err := engine.Save()
			if err != nil {
				if err.Error() == "no wallpaper currently set" {
					out.Warning("No wallpaper currently set")
					return nil
				}
				out.Error("Failed to save: %v", err)
				return err
			}

			if !engine.IsTempWallpaper() && result.Path == engine.CurrentPath() {
				out.Info("Current wallpaper is already saved")
				out.Field("Path", shortenPath(result.Path))
				return nil
			}

			if dryRun {
				out.Info("Would save: %s", shortenPath(result.Path))
				return nil
			}

			out.Success("Wallpaper saved")
			out.Field("Path", shortenPath(result.Path))

			return nil
		},
	}
}

// newShowCmd creates the show command.
func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Open current wallpaper in default viewer",
		RunE: func(cmd *cobra.Command, args []string) error {
			initOutput()

			engine, err := newEngine()
			if err != nil {
				out.ErrorWithHint(err.Error(), "Run 'wallboy init' to create a default configuration")
				return err
			}

			if err := engine.OpenImage(); err != nil {
				if err.Error() == "no wallpaper currently set" {
					out.Warning("No wallpaper currently set")
					return nil
				}
				out.Error("Failed to open image: %v", err)
				return err
			}

			return nil
		},
	}
}

// newOpenCmd creates the open command.
func newOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open",
		Short: "Reveal current wallpaper in Finder",
		RunE: func(cmd *cobra.Command, args []string) error {
			initOutput()

			engine, err := newEngine()
			if err != nil {
				out.ErrorWithHint(err.Error(), "Run 'wallboy init' to create a default configuration")
				return err
			}

			if err := engine.OpenInFinder(); err != nil {
				if err.Error() == "no wallpaper currently set" {
					out.Warning("No wallpaper currently set")
					return nil
				}
				out.Error("Failed to open Finder: %v", err)
				return err
			}

			out.Success("Opened in Finder")
			return nil
		},
	}
}

// newInfoCmd creates the info command.
func newInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show information about current wallpaper",
		RunE: func(cmd *cobra.Command, args []string) error {
			initOutput()

			engine, err := newEngine()
			if err != nil {
				out.ErrorWithHint(err.Error(), "Run 'wallboy init' to create a default configuration")
				return err
			}

			info, err := engine.Info()
			if err != nil {
				if err.Error() == "no wallpaper currently set" {
					out.Warning("No wallpaper currently set")
					return nil
				}
				out.Error("Failed to get info: %v", err)
				return err
			}

			out.Print("")
			out.Field("Path", info.Path)
			out.Field("Theme", info.Theme)
			out.Field("Source", info.SourceID)
			out.Field("Set at", info.SetAt.Format("2006-01-02 15:04:05"))
			if info.IsTemp {
				out.FieldColored("Status", "temporary (use 'save' to keep)", ui.Yellow)
			} else {
				out.FieldColored("Status", "saved", ui.Green)
			}
			out.Print("")

			if !info.Exists {
				out.Warning("File no longer exists")
			}

			return nil
		},
	}
}

// newColorsCmd creates the colors command.
func newColorsCmd() *cobra.Command {
	var topN int

	cmd := &cobra.Command{
		Use:   "colors",
		Short: "Show dominant colors of current wallpaper",
		RunE: func(cmd *cobra.Command, args []string) error {
			initOutput()

			engine, err := newEngine()
			if err != nil {
				out.ErrorWithHint(err.Error(), "Run 'wallboy init' to create a default configuration")
				return err
			}

			spinner := ui.NewSpinner(out, "Analyzing colors...")
			spinner.Start()

			colors, err := engine.AnalyzeColors(topN)
			spinner.Stop()

			if err != nil {
				if err.Error() == "no wallpaper currently set" {
					out.Warning("No wallpaper currently set")
					return nil
				}
				out.Error("Failed to analyze colors: %v", err)
				return err
			}

			out.Print("")
			for _, c := range colors {
				out.ColorSwatch(c.Hex())
			}
			out.Print("")

			return nil
		},
	}

	cmd.Flags().IntVar(&topN, "top", 10, "number of colors to show")

	return cmd
}

// newDeleteCmd creates the delete command.
func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete",
		Short: "Delete current wallpaper and set next",
		Long: `Deletes the current wallpaper file (only remote/temp, not local sources)
and automatically sets the next random wallpaper.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			initOutput()

			engine, err := newEngine()
			if err != nil {
				out.ErrorWithHint(err.Error(), "Run 'wallboy init' to create a default configuration")
				return err
			}

			info, _ := engine.Info()
			if info == nil {
				out.Warning("No wallpaper currently set")
				return nil
			}

			if dryRun {
				out.Info("Would delete: %s", shortenPath(info.Path))
				out.Info("Would set next wallpaper")
				return nil
			}

			result, err := engine.Delete(cmd.Context())
			if err != nil {
				out.Error("Failed to delete: %v", err)
				return err
			}

			out.Success("Deleted previous wallpaper")
			out.WallpaperInfo(result.Theme, result.SourceID, shortenPath(result.Path), result.SetAt)

			if result.IsTemp {
				out.Print("")
				out.Info("Use 'wallboy save' to keep this wallpaper")
			}

			return nil
		},
	}
}

// newSourcesCmd creates the sources command.
func newSourcesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sources",
		Short: "List available datasources",
		RunE: func(cmd *cobra.Command, args []string) error {
			initOutput()

			// Load config directly for sources list (no engine needed)
			cfg, err := config.Load(cfgFile)
			if err != nil {
				out.Error("Failed to load config: %v", err)
				return err
			}

			allSources := cfg.GetAllDatasources()
			if len(allSources) == 0 {
				out.Warning("No datasources configured")
				out.Info("Edit your config file to add datasources")
				return nil
			}

			headers := []string{"ID", "Theme", "Type", "Provider / Path"}
			var rows [][]string

			for _, s := range allSources {
				desc := ""
				if s.Datasource.Type == config.DatasourceTypeLocal {
					desc = shortenPath(s.Datasource.Dir)
				} else {
					desc = string(s.Datasource.Provider)
				}

				rows = append(rows, []string{
					s.Datasource.ID,
					string(s.Theme),
					string(s.Datasource.Type),
					desc,
				})
			}

			out.Print("")
			out.Table(headers, rows)
			out.Print("")

			return nil
		},
	}
}

// newVersionCmd creates the version command.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			initOutput()
			out.Print("wallboy version 0.1.0")
		},
	}
}

// newAgentInstallCmd creates the agent-install command.
func newAgentInstallCmd() *cobra.Command {
	var interval int

	cmd := &cobra.Command{
		Use:   "agent-install",
		Short: "Install background agent for auto-rotation",
		Long:  "Installs a background agent that runs 'wallboy next' at regular intervals.",
		RunE: func(cmd *cobra.Command, args []string) error {
			initOutput()

			// Validate interval
			if interval < minInterval {
				out.Error("Minimum interval is %d seconds", minInterval)
				return fmt.Errorf("interval too small")
			}

			engine, err := newEngine()
			if err != nil {
				out.ErrorWithHint(err.Error(), "Run 'wallboy init' to create a default configuration")
				return err
			}

			if err := engine.InstallAgent(time.Duration(interval) * time.Second); err != nil {
				out.Error("Failed to install agent: %v", err)
				return err
			}

			out.Success("Agent installed")
			out.Field("Interval", formatDuration(time.Duration(interval)*time.Second))
			out.Field("Log", shortenPath(filepath.Join(config.DefaultConfigDir(), "agent.log")))

			return nil
		},
	}

	cmd.Flags().IntVar(&interval, "interval", defaultInterval, "interval in seconds (minimum 60)")

	return cmd
}

// newAgentUninstallCmd creates the agent-uninstall command.
func newAgentUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "agent-uninstall",
		Short: "Uninstall background agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			initOutput()

			engine, err := newEngine()
			if err != nil {
				out.ErrorWithHint(err.Error(), "Run 'wallboy init' to create a default configuration")
				return err
			}

			status, err := engine.AgentStatus()
			if err != nil {
				out.Error("Failed to get agent status: %v", err)
				return err
			}

			if !status.Installed {
				out.Info("Agent is not installed")
				return nil
			}

			if err := engine.UninstallAgent(); err != nil {
				out.Error("Failed to uninstall agent: %v", err)
				return err
			}

			out.Success("Agent uninstalled")
			return nil
		},
	}
}

// newAgentStatusCmd creates the agent-status command.
func newAgentStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "agent-status",
		Short: "Show agent status",
		RunE: func(cmd *cobra.Command, args []string) error {
			initOutput()

			engine, err := newEngine()
			if err != nil {
				out.ErrorWithHint(err.Error(), "Run 'wallboy init' to create a default configuration")
				return err
			}

			status, err := engine.AgentStatus()
			if err != nil {
				out.Error("Failed to get agent status: %v", err)
				return err
			}

			if !status.Supported {
				out.Error("Background scheduling is not supported on this platform")
				return fmt.Errorf("scheduler not supported")
			}

			if !status.Installed {
				out.Info("Agent is not installed")
				return nil
			}

			if status.Running {
				out.Success("Agent is running")
				if status.Interval > 0 {
					out.Field("Interval", formatDuration(status.Interval))
				}
				out.Field("Log", shortenPath(status.LogPath))
			} else {
				out.Warning("Agent is installed but not running")
				out.Field("Log", shortenPath(status.LogPath))
			}

			return nil
		},
	}
}

// shortenPath shortens a path for display.
func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if len(path) > len(home) && path[:len(home)] == home {
		return "~" + path[len(home):]
	}
	return path
}

// formatDuration formats duration to human readable (e.g., "1.5 minutes")
func formatDuration(d time.Duration) string {
	minutes := d.Minutes()
	if minutes == 1 {
		return "1 minute"
	}
	if minutes == float64(int(minutes)) {
		return fmt.Sprintf("%d minutes", int(minutes))
	}
	return fmt.Sprintf("%.1f minutes", minutes)
}
