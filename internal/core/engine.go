package core

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/Artawower/wallboy/internal/colors"
	"github.com/Artawower/wallboy/internal/config"
	"github.com/Artawower/wallboy/internal/datasource"
	"github.com/Artawower/wallboy/internal/platform"
	"github.com/Artawower/wallboy/internal/state"
)

// Engine is the main wallpaper management engine.
type Engine struct {
	config   *config.Config
	state    *state.State
	platform platform.Platform
	manager  *datasource.Manager

	// Options
	themeOverride  string
	sourceOverride string
	dryRun         bool
}

// Option is a function that configures the Engine.
type Option func(*Engine)

// WithThemeOverride sets a theme override.
func WithThemeOverride(theme string) Option {
	return func(e *Engine) {
		e.themeOverride = theme
	}
}

// WithSourceOverride sets a source override.
func WithSourceOverride(source string) Option {
	return func(e *Engine) {
		e.sourceOverride = source
	}
}

// WithDryRun enables dry-run mode.
func WithDryRun(dryRun bool) Option {
	return func(e *Engine) {
		e.dryRun = dryRun
	}
}

// New creates a new Engine instance.
func New(configPath string, opts ...Option) (*Engine, error) {
	// Load config
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Ensure directories exist
	if err := cfg.EnsureDirectories(); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	// Load state
	st, err := state.Load(cfg.State.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	e := &Engine{
		config:   cfg,
		state:    st,
		platform: platform.Current(),
	}

	// Apply options
	for _, opt := range opts {
		opt(e)
	}

	// Initialize datasource manager
	if err := e.initManager(); err != nil {
		return nil, err
	}

	return e, nil
}

// initManager initializes the datasource manager.
func (e *Engine) initManager() error {
	currentTheme := e.detectTheme()
	uploadDir := e.config.GetUploadDir(currentTheme.ToConfigMode())
	tempDir := config.GetTempDir()
	e.manager = datasource.NewManager(uploadDir, tempDir)

	if e.sourceOverride != "" {
		// Add only the specified source
		ds, dsTheme, err := e.config.FindDatasource(e.sourceOverride)
		if err != nil {
			return fmt.Errorf("datasource not found: %s", e.sourceOverride)
		}
		e.addDatasource(ds, string(dsTheme), e.config.GetUploadDir(dsTheme), tempDir)
	} else {
		// Add all datasources for current theme
		themeConfig := e.config.GetThemeConfig(currentTheme.ToConfigMode())
		for _, ds := range themeConfig.Datasources {
			e.addDatasource(&ds, string(currentTheme), uploadDir, tempDir)
		}
	}

	return nil
}

// addDatasource adds a datasource to the manager.
func (e *Engine) addDatasource(ds *config.Datasource, themeName, uploadDir, tempDir string) {
	switch ds.Type {
	case config.DatasourceTypeLocal:
		e.manager.AddSource(datasource.NewLocalSource(*ds, themeName))
	case config.DatasourceTypeRemote:
		e.manager.AddSource(datasource.NewRemoteSource(*ds, themeName, uploadDir, tempDir))
	}
}

// detectTheme detects the current theme.
func (e *Engine) detectTheme() Theme {
	// Check for override
	if e.themeOverride != "" {
		switch e.themeOverride {
		case "light":
			return ThemeLight
		case "dark":
			return ThemeDark
		}
	}

	// Check config mode
	switch e.config.Theme.Mode {
	case config.ThemeModeLight:
		return ThemeLight
	case config.ThemeModeDark:
		return ThemeDark
	case config.ThemeModeAuto:
		return FromPlatformTheme(e.platform.Theme().Detect())
	default:
		return ThemeLight
	}
}

// ToConfigMode converts Theme to config.ThemeMode.
func (t Theme) ToConfigMode() config.ThemeMode {
	switch t {
	case ThemeLight:
		return config.ThemeModeLight
	case ThemeDark:
		return config.ThemeModeDark
	default:
		return config.ThemeModeLight
	}
}

// Next sets the next random wallpaper.
func (e *Engine) Next(ctx context.Context) (*WallpaperResult, error) {
	currentTheme := e.detectTheme()

	// Clean up previous temp file if it was temporary
	if e.state.HasCurrent() && e.state.IsTempWallpaper() {
		os.Remove(e.state.Current.Path)
	}

	var img *datasource.Image
	var isTemp bool
	var err error

	// Pick image based on whether source was specified
	if e.sourceOverride != "" {
		img, isTemp, err = e.pickNextImageFromSource(ctx, e.sourceOverride)
	} else {
		img, isTemp, err = e.pickNextImage(ctx, string(currentTheme))
	}
	if err != nil {
		return nil, err
	}

	if e.dryRun {
		return &WallpaperResult{
			Path:     img.Path,
			Theme:    img.Theme,
			SourceID: img.SourceID,
			IsTemp:   isTemp,
			SetAt:    time.Now(),
		}, nil
	}

	// Set wallpaper
	if err := e.platform.Wallpaper().Set(img.Path); err != nil {
		return nil, fmt.Errorf("failed to set wallpaper: %w", err)
	}

	// Update state
	e.state.SetCurrent(img.Path, img.SourceID, img.Theme, isTemp)
	if err := e.state.Save(); err != nil {
		// Non-fatal error
		_ = err
	}

	return &WallpaperResult{
		Path:     img.Path,
		Theme:    img.Theme,
		SourceID: img.SourceID,
		IsTemp:   isTemp,
		SetAt:    e.state.Current.SetAt,
	}, nil
}

// pickNextImage picks the next image from available sources.
func (e *Engine) pickNextImage(ctx context.Context, themeName string) (*datasource.Image, bool, error) {
	sources := e.manager.GetSourcesByTheme(themeName)
	if len(sources) == 0 {
		return nil, false, fmt.Errorf("no datasources available")
	}

	// Separate local and remote sources
	var localSources []datasource.Source
	var remoteSources []*datasource.RemoteSource
	for _, s := range sources {
		if s.Type() == config.DatasourceTypeLocal {
			localSources = append(localSources, s)
		} else if remote, ok := s.(*datasource.RemoteSource); ok {
			remoteSources = append(remoteSources, remote)
		}
	}

	// If we have both local and remote, randomly pick which type to use
	useRemote := false
	if len(localSources) > 0 && len(remoteSources) > 0 {
		useRemote = rand.Intn(2) == 0
	} else if len(remoteSources) > 0 {
		useRemote = true
	}

	// Remote: always fetch new image, with fallback to other sources
	if useRemote {
		// Shuffle remote sources for random order
		shuffledRemotes := make([]*datasource.RemoteSource, len(remoteSources))
		copy(shuffledRemotes, remoteSources)
		rand.Shuffle(len(shuffledRemotes), func(i, j int) {
			shuffledRemotes[i], shuffledRemotes[j] = shuffledRemotes[j], shuffledRemotes[i]
		})

		// Try each remote source
		var lastErr error
		for _, remote := range shuffledRemotes {
			img, err := remote.FetchRandom(ctx)
			if err == nil {
				return img, true, nil
			}
			lastErr = err
		}

		// All remotes failed, fallback to local if available
		if len(localSources) > 0 {
			img, err := e.manager.PickRandom(ctx, themeName, e.state.History)
			if err == nil {
				return img, false, nil
			}
		}

		return nil, false, fmt.Errorf("failed to fetch from remote: %w", lastErr)
	}

	// Local: pick from existing images
	img, err := e.manager.PickRandom(ctx, themeName, e.state.History)
	if err != nil {
		// No local images, try remote as fallback
		for _, remote := range remoteSources {
			img, err := remote.FetchRandom(ctx)
			if err == nil {
				return img, true, nil
			}
		}
		return nil, false, fmt.Errorf("failed to pick image: %w", err)
	}

	return img, false, nil
}

// pickNextImageFromSource picks an image from a specific source.
func (e *Engine) pickNextImageFromSource(ctx context.Context, sourceID string) (*datasource.Image, bool, error) {
	source, err := e.manager.GetSourceByID(sourceID)
	if err != nil {
		return nil, false, fmt.Errorf("source not found: %s", sourceID)
	}

	// Remote: always fetch new
	if source.Type() == config.DatasourceTypeRemote {
		img, err := e.manager.FetchRandomFromRemote(ctx, sourceID)
		if err != nil {
			return nil, false, fmt.Errorf("failed to fetch from remote: %w", err)
		}
		return img, true, nil
	}

	// Local: pick from existing
	img, err := e.manager.PickRandomFromSource(ctx, sourceID, e.state.History)
	if err != nil {
		return nil, false, fmt.Errorf("failed to pick image: %w", err)
	}

	return img, false, nil
}

// Save saves the current wallpaper permanently.
func (e *Engine) Save() (*WallpaperResult, error) {
	if !e.state.HasCurrent() {
		return nil, fmt.Errorf("no wallpaper currently set")
	}

	if !e.state.IsTempWallpaper() {
		return &WallpaperResult{
			Path:     e.state.Current.Path,
			Theme:    e.state.Current.Theme,
			SourceID: e.state.Current.SourceID,
			IsTemp:   false,
			SetAt:    e.state.Current.SetAt,
		}, nil
	}

	// Get the remote source to save through
	remote, err := e.manager.GetRemoteSource(e.state.Current.SourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	if e.dryRun {
		return &WallpaperResult{
			Path:     e.state.Current.Path,
			Theme:    e.state.Current.Theme,
			SourceID: e.state.Current.SourceID,
			IsTemp:   true,
			SetAt:    e.state.Current.SetAt,
		}, nil
	}

	// Save the image
	newPath, err := remote.Save(e.state.Current.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to save wallpaper: %w", err)
	}

	// Update state
	e.state.MarkSaved(newPath)
	if err := e.state.Save(); err != nil {
		_ = err
	}

	return &WallpaperResult{
		Path:     newPath,
		Theme:    e.state.Current.Theme,
		SourceID: e.state.Current.SourceID,
		IsTemp:   false,
		SetAt:    e.state.Current.SetAt,
	}, nil
}

// Delete deletes the current wallpaper and sets the next one.
func (e *Engine) Delete(ctx context.Context) (*WallpaperResult, error) {
	if !e.state.HasCurrent() {
		return nil, fmt.Errorf("no wallpaper currently set")
	}

	currentPath := e.state.Current.Path
	currentSource := e.state.Current.SourceID

	// Check if we should delete (only remote/temp files, not local sources)
	shouldDelete := false
	source, err := e.manager.GetSourceByID(currentSource)
	if err == nil && source.Type() == config.DatasourceTypeRemote {
		shouldDelete = true
	}
	if e.state.IsTempWallpaper() {
		shouldDelete = true
	}

	if e.dryRun {
		return &WallpaperResult{
			Path:     currentPath,
			Theme:    e.state.Current.Theme,
			SourceID: currentSource,
			IsTemp:   e.state.IsTempWallpaper(),
			SetAt:    e.state.Current.SetAt,
		}, nil
	}

	// Delete file if from remote/temp
	if shouldDelete {
		os.Remove(currentPath)
	}

	// Set next wallpaper
	return e.Next(ctx)
}

// Info returns information about the current wallpaper.
func (e *Engine) Info() (*WallpaperInfo, error) {
	if !e.state.HasCurrent() {
		return nil, fmt.Errorf("no wallpaper currently set")
	}

	_, err := os.Stat(e.state.Current.Path)
	exists := err == nil

	return &WallpaperInfo{
		Path:     e.state.Current.Path,
		Theme:    e.state.Current.Theme,
		SourceID: e.state.Current.SourceID,
		IsTemp:   e.state.IsTempWallpaper(),
		SetAt:    e.state.Current.SetAt,
		Exists:   exists,
	}, nil
}

// AnalyzeColors analyzes the colors in the current wallpaper.
func (e *Engine) AnalyzeColors(topN int) ([]Color, error) {
	if !e.state.HasCurrent() {
		return nil, fmt.Errorf("no wallpaper currently set")
	}

	if _, err := os.Stat(e.state.Current.Path); os.IsNotExist(err) {
		return nil, fmt.Errorf("wallpaper file not found")
	}

	result, err := colors.Analyze(e.state.Current.Path, topN)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze colors: %w", err)
	}

	coreColors := make([]Color, len(result))
	for i, c := range result {
		coreColors[i] = Color{R: c.R, G: c.G, B: c.B}
	}

	return coreColors, nil
}

// ListSources returns all available datasources.
func (e *Engine) ListSources() []SourceInfo {
	allSources := e.config.GetAllDatasources()
	result := make([]SourceInfo, len(allSources))

	for i, s := range allSources {
		desc := ""
		if s.Datasource.Type == config.DatasourceTypeLocal {
			desc = s.Datasource.Dir
		} else {
			desc = string(s.Datasource.Provider)
		}

		result[i] = SourceInfo{
			ID:          s.Datasource.ID,
			Theme:       string(s.Theme),
			Type:        string(s.Datasource.Type),
			Description: desc,
		}
	}

	return result
}

// CurrentPath returns the current wallpaper path, if any.
func (e *Engine) CurrentPath() string {
	if e.state.HasCurrent() {
		return e.state.Current.Path
	}
	return ""
}

// IsTempWallpaper returns true if the current wallpaper is temporary.
func (e *Engine) IsTempWallpaper() bool {
	return e.state.IsTempWallpaper()
}

// GetCurrentWallpaperPath returns the actual current wallpaper path from the system.
// This is more reliable than state when wallpaper may have been changed externally.
func (e *Engine) GetCurrentWallpaperPath() (string, error) {
	return e.platform.Wallpaper().Get()
}

// OpenInFinder opens the current wallpaper in the file manager.
// Uses the actual system wallpaper path, not the cached state.
func (e *Engine) OpenInFinder() error {
	path, err := e.platform.Wallpaper().Get()
	if err != nil {
		return fmt.Errorf("failed to get current wallpaper: %w", err)
	}
	return e.platform.FileManager().Reveal(path)
}

// OpenImage opens the current wallpaper in the default viewer.
// Uses the actual system wallpaper path, not the cached state.
func (e *Engine) OpenImage() error {
	path, err := e.platform.Wallpaper().Get()
	if err != nil {
		return fmt.Errorf("failed to get current wallpaper: %w", err)
	}
	return e.platform.FileManager().Open(path)
}

// Agent methods

const agentLabel = "com.wallboy.agent"

// InstallAgent installs the background agent.
func (e *Engine) InstallAgent(interval time.Duration) error {
	scheduler := e.platform.Scheduler()
	if !scheduler.IsSupported() {
		return fmt.Errorf("scheduler not supported on %s", e.platform.Name())
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	logPath := filepath.Join(config.DefaultConfigDir(), "agent.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	cfg := platform.SchedulerConfig{
		Label:     agentLabel,
		Command:   execPath,
		Args:      []string{"next"},
		Interval:  interval,
		RunAtLoad: true,
		LogPath:   logPath,
	}

	return scheduler.Install(cfg)
}

// UninstallAgent uninstalls the background agent.
func (e *Engine) UninstallAgent() error {
	scheduler := e.platform.Scheduler()
	if !scheduler.IsSupported() {
		return fmt.Errorf("scheduler not supported on %s", e.platform.Name())
	}
	return scheduler.Uninstall(agentLabel)
}

// AgentStatus returns the status of the background agent.
func (e *Engine) AgentStatus() (*AgentStatus, error) {
	scheduler := e.platform.Scheduler()

	status := &AgentStatus{
		Supported: scheduler.IsSupported(),
		LogPath:   filepath.Join(config.DefaultConfigDir(), "agent.log"),
	}

	if !scheduler.IsSupported() {
		return status, nil
	}

	platformStatus, err := scheduler.Status(agentLabel)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent status: %w", err)
	}

	status.Installed = platformStatus.Installed
	status.Running = platformStatus.Running
	status.Interval = platformStatus.Interval

	return status, nil
}

// Platform returns the current platform.
func (e *Engine) Platform() platform.Platform {
	return e.platform
}

// Config returns the current config.
func (e *Engine) Config() *config.Config {
	return e.config
}
