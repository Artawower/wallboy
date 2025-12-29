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

type Engine struct {
	config   *config.Config
	state    *state.State
	platform platform.Platform
	manager  *datasource.Manager

	themeOverride    string
	providerOverride string
	queryOverride    string
	dryRun           bool
}

type Option func(*Engine)

func WithThemeOverride(theme string) Option {
	return func(e *Engine) { e.themeOverride = theme }
}

func WithProviderOverride(provider string) Option {
	return func(e *Engine) { e.providerOverride = provider }
}

func WithDryRun(dryRun bool) Option {
	return func(e *Engine) { e.dryRun = dryRun }
}

func WithQueryOverride(query string) Option {
	return func(e *Engine) { e.queryOverride = query }
}

func New(configPath string, opts ...Option) (*Engine, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.EnsureDirectories(); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	st, err := state.Load(cfg.State.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	e := &Engine{
		config:   cfg,
		state:    st,
		platform: platform.Current(),
	}

	for _, opt := range opts {
		opt(e)
	}

	e.initManager()

	return e, nil
}

func (e *Engine) initManager() {
	theme := e.detectTheme()
	themeMode := theme.ToConfigMode()
	uploadDir := e.config.GetUploadDir(themeMode)
	tempDir := config.GetTempDir()

	e.manager = datasource.NewManager(uploadDir, tempDir)

	localConfig := e.config.GetLocalConfig()
	for i, dir := range e.config.GetLocalDirs(themeMode) {
		id := fmt.Sprintf("%s-local-%d", theme, i+1)
		source := datasource.NewLocalSource(id, dir, string(theme), localConfig.Recursive)
		e.manager.AddLocalSource(source)
	}

	queries := e.config.GetQueries(themeMode)
	for name, providerCfg := range e.config.GetRemoteProviders(themeMode) {
		id := fmt.Sprintf("%s-%s", theme, name)
		source := datasource.NewRemoteSource(id, name, providerCfg.Auth, string(theme), uploadDir, tempDir, queries)
		e.manager.AddRemoteSource(source)
	}
}

func (e *Engine) detectTheme() Theme {
	if e.themeOverride != "" {
		switch e.themeOverride {
		case "light":
			return ThemeLight
		case "dark":
			return ThemeDark
		}
	}

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

func (e *Engine) Next(ctx context.Context) (*WallpaperResult, error) {
	currentTheme := e.detectTheme()
	themeName := string(currentTheme)

	// Clean up previous temp wallpaper
	if e.state.HasCurrent() && e.state.IsTempWallpaper() {
		os.Remove(e.state.Current.Path)
	}

	var img *datasource.Image
	var isTemp bool
	var err error
	var usedPrefetch bool

	// Build cache key for prefetch lookup
	cacheKey := e.buildCacheKey(themeName)

	// Check if we should use remote (for prefetch logic)
	willUseRemote := e.willUseRemote(themeName)

	// Try to use prefetched wallpaper (only for remote sources)
	if willUseRemote {
		if prefetched := e.state.GetPrefetched(cacheKey); prefetched != nil {
			img = &datasource.Image{
				Path:     prefetched.Path,
				SourceID: prefetched.SourceID,
				Theme:    themeName,
				IsLocal:  false,
			}
			isTemp = true
			usedPrefetch = true
			e.state.ClearPrefetched()
		}
	}

	// If no prefetch available, fetch normally
	if img == nil {
		if e.providerOverride != "" {
			img, isTemp, err = e.pickFromProvider(ctx, themeName, e.providerOverride)
		} else if e.queryOverride != "" {
			img, isTemp, err = e.pickFromRemote(ctx, themeName)
		} else {
			img, isTemp, err = e.pickNext(ctx, themeName)
		}

		if err != nil {
			return nil, err
		}
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

	e.state.SetCurrent(img.Path, img.SourceID, img.Theme, isTemp)

	// Prefetch next wallpaper for remote sources
	if willUseRemote || usedPrefetch {
		e.prefetchNext(ctx, themeName, cacheKey)
	}

	_ = e.state.Save()

	return &WallpaperResult{
		Path:     img.Path,
		Theme:    img.Theme,
		SourceID: img.SourceID,
		IsTemp:   isTemp,
		SetAt:    e.state.Current.SetAt,
	}, nil
}

// buildCacheKey creates a cache key for prefetch lookup.
// Format: "theme:provider:query"
func (e *Engine) buildCacheKey(theme string) string {
	provider := e.providerOverride
	query := e.queryOverride
	return fmt.Sprintf("%s:%s:%s", theme, provider, query)
}

// willUseRemote determines if the next fetch will DEFINITELY use remote sources.
// Returns true only when we're certain remote will be used (for prefetch logic).
func (e *Engine) willUseRemote(theme string) bool {
	// Explicit provider override (not local)
	if e.providerOverride != "" {
		return e.providerOverride != "local"
	}

	// Query override forces remote
	if e.queryOverride != "" {
		return true
	}

	// If we have both local and remote, pickNext() uses random 50/50
	// so we can't guarantee remote will be used - don't prefetch
	hasLocal := e.manager.HasLocalSources(theme)
	hasRemote := e.manager.HasRemoteSources(theme)

	// Only prefetch when remote is the ONLY option
	return hasRemote && !hasLocal
}

// prefetchNext fetches the next wallpaper and stores it for later use.
func (e *Engine) prefetchNext(ctx context.Context, theme, cacheKey string) {
	var img *datasource.Image
	var err error

	if e.providerOverride != "" {
		img, _, err = e.pickFromProvider(ctx, theme, e.providerOverride)
	} else if e.queryOverride != "" {
		img, err = e.manager.FetchRandomRemote(ctx, theme, e.queryOverride)
	} else {
		img, err = e.manager.FetchRandomRemote(ctx, theme, "")
	}

	if err != nil {
		// Prefetch failed - not critical, just skip
		return
	}

	e.state.SetPrefetched(img.Path, img.SourceID, cacheKey)
}

func (e *Engine) pickNext(ctx context.Context, theme string) (*datasource.Image, bool, error) {
	hasLocal := e.manager.HasLocalSources(theme)
	hasRemote := e.manager.HasRemoteSources(theme)

	if !hasLocal && !hasRemote {
		return nil, false, fmt.Errorf("no sources available for theme: %s", theme)
	}

	useRemote := false
	if hasLocal && hasRemote {
		useRemote = rand.Intn(2) == 0
	} else if hasRemote {
		useRemote = true
	}

	if useRemote {
		img, err := e.manager.FetchRandomRemote(ctx, theme, e.queryOverride)
		if err == nil {
			return img, true, nil
		}
		if hasLocal {
			img, err := e.manager.PickRandomLocal(ctx, theme, e.state.History)
			if err == nil {
				return img, false, nil
			}
		}
		return nil, false, fmt.Errorf("failed to fetch from remote: %w", err)
	}

	img, err := e.manager.PickRandomLocal(ctx, theme, e.state.History)
	if err != nil {
		if hasRemote {
			img, err := e.manager.FetchRandomRemote(ctx, theme, e.queryOverride)
			if err == nil {
				return img, true, nil
			}
		}
		return nil, false, fmt.Errorf("failed to pick image: %w", err)
	}

	return img, false, nil
}

func (e *Engine) pickFromProvider(ctx context.Context, theme, providerName string) (*datasource.Image, bool, error) {
	// Handle "local" provider specially
	if providerName == "local" {
		img, err := e.manager.PickRandomLocal(ctx, theme, e.state.History)
		if err != nil {
			return nil, false, fmt.Errorf("failed to pick from local: %w", err)
		}
		return img, false, nil
	}

	// Try to find existing source for this provider
	img, err := e.manager.FetchFromProvider(ctx, theme, providerName, e.queryOverride)
	if err == nil {
		return img, true, nil
	}

	// Provider not in manager - try to create it on-the-fly
	// This allows --provider bing to work even if bing is not in config
	themeMode := e.detectTheme().ToConfigMode()
	uploadDir := e.config.GetUploadDir(themeMode)
	tempDir := config.GetTempDir()

	// Get auth from config if provider exists there, otherwise empty (works for bing)
	var auth string
	if providerCfg, exists := e.config.Providers[providerName]; exists {
		auth = providerCfg.Auth
	}

	// Create temporary source for this request
	source := datasource.NewRemoteSource(
		fmt.Sprintf("%s-%s", theme, providerName),
		providerName,
		auth,
		theme,
		uploadDir,
		tempDir,
		e.config.GetQueries(themeMode),
	)

	img, err = source.FetchRandom(ctx, e.queryOverride)
	if err != nil {
		return nil, false, fmt.Errorf("failed to fetch from %s: %w", providerName, err)
	}
	return img, true, nil
}

func (e *Engine) pickFromRemote(ctx context.Context, theme string) (*datasource.Image, bool, error) {
	if !e.manager.HasRemoteSources(theme) {
		return e.pickNext(ctx, theme)
	}

	img, err := e.manager.FetchRandomRemote(ctx, theme, e.queryOverride)
	if err != nil {
		return nil, false, fmt.Errorf("failed to fetch from remote: %w", err)
	}
	return img, true, nil
}

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

	remote, err := e.manager.GetRemoteSourceByID(e.state.Current.SourceID)
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

	newPath, err := remote.Save(e.state.Current.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to save wallpaper: %w", err)
	}

	e.state.MarkSaved(newPath)
	_ = e.state.Save()

	return &WallpaperResult{
		Path:     newPath,
		Theme:    e.state.Current.Theme,
		SourceID: e.state.Current.SourceID,
		IsTemp:   false,
		SetAt:    e.state.Current.SetAt,
	}, nil
}

func (e *Engine) Delete(ctx context.Context) (*WallpaperResult, error) {
	if !e.state.HasCurrent() {
		return nil, fmt.Errorf("no wallpaper currently set")
	}

	currentPath := e.state.Current.Path

	shouldDelete := e.state.IsTempWallpaper()

	if e.dryRun {
		return &WallpaperResult{
			Path:     currentPath,
			Theme:    e.state.Current.Theme,
			SourceID: e.state.Current.SourceID,
			IsTemp:   e.state.IsTempWallpaper(),
			SetAt:    e.state.Current.SetAt,
		}, nil
	}

	if shouldDelete {
		os.Remove(currentPath)
	}

	return e.Next(ctx)
}

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

func (e *Engine) ListSources() []SourceInfo {
	theme := e.detectTheme()
	themeName := string(theme)
	var result []SourceInfo

	for _, s := range e.manager.GetLocalSources(themeName) {
		result = append(result, SourceInfo{
			ID:          s.ID(),
			Theme:       themeName,
			Type:        "local",
			Description: s.Description(),
		})
	}

	for _, s := range e.manager.GetRemoteSources(themeName) {
		result = append(result, SourceInfo{
			ID:          s.ID(),
			Theme:       themeName,
			Type:        "remote",
			Description: s.Description(),
		})
	}

	return result
}

func (e *Engine) CurrentPath() string {
	if e.state.HasCurrent() {
		return e.state.Current.Path
	}
	return ""
}

func (e *Engine) IsTempWallpaper() bool {
	return e.state.IsTempWallpaper()
}

func (e *Engine) GetCurrentWallpaperPath() (string, error) {
	return e.platform.Wallpaper().Get()
}

// getWallpaperPath returns the best known wallpaper path.
// Prefers state path (if exists), falls back to system query.
func (e *Engine) getWallpaperPath() string {
	// First try state - it's more reliable
	if e.state != nil && e.state.HasCurrent() {
		return e.state.Current.Path
	}
	// Fall back to system
	if e.platform == nil {
		return ""
	}
	path, err := e.platform.Wallpaper().Get()
	if err != nil {
		return ""
	}
	return path
}

func (e *Engine) OpenInFinder() error {
	path := e.getWallpaperPath()
	if path == "" {
		return fmt.Errorf("no wallpaper path available")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("wallpaper file no longer exists: %s", path)
	}
	return e.platform.FileManager().Reveal(path)
}

func (e *Engine) OpenImage() error {
	path := e.getWallpaperPath()
	if path == "" {
		return fmt.Errorf("no wallpaper path available")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("wallpaper file no longer exists: %s", path)
	}
	return e.platform.FileManager().Open(path)
}

const agentLabel = "com.wallboy.agent"

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

func (e *Engine) UninstallAgent() error {
	scheduler := e.platform.Scheduler()
	if !scheduler.IsSupported() {
		return fmt.Errorf("scheduler not supported on %s", e.platform.Name())
	}
	return scheduler.Uninstall(agentLabel)
}

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

func (e *Engine) Platform() platform.Platform {
	return e.platform
}

func (e *Engine) Config() *config.Config {
	return e.config
}
