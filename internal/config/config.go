package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type ThemeMode string

const (
	ThemeModeAuto  ThemeMode = "auto"
	ThemeModeLight ThemeMode = "light"
	ThemeModeDark  ThemeMode = "dark"
)

type ProviderType string

const (
	ProviderUnsplash  ProviderType = "unsplash"
	ProviderWallhaven ProviderType = "wallhaven"
	ProviderWallhalla ProviderType = "wallhalla"
	ProviderBing      ProviderType = "bing"
	ProviderLocal     ProviderType = "local"
)

type ProviderConfig struct {
	Auth      string `toml:"auth"`
	Recursive bool   `toml:"recursive"`
	Weight    int    `toml:"weight"`
}

type ThemeConfig struct {
	Dirs      []string `toml:"dirs"`
	UploadDir string   `toml:"upload-dir"`
	Queries   []string `toml:"queries"`
	Providers []string `toml:"providers"`
}

type StateConfig struct {
	Path string `toml:"path"`
}

type ThemeSettings struct {
	Mode ThemeMode `toml:"mode"`
}

type Config struct {
	State     StateConfig               `toml:"state"`
	Theme     ThemeSettings             `toml:"theme"`
	Providers map[string]ProviderConfig `toml:"providers"`
	Light     ThemeConfig               `toml:"light"`
	Dark      ThemeConfig               `toml:"dark"`

	configPath string
}

func DefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "wallboy")
}

func DefaultConfig() *Config {
	configDir := DefaultConfigDir()
	home, _ := os.UserHomeDir()
	picturesDir := filepath.Join(home, "Pictures")

	return &Config{
		State: StateConfig{
			Path: filepath.Join(configDir, "state.json"),
		},
		Theme: ThemeSettings{
			Mode: ThemeModeAuto,
		},
		Providers: map[string]ProviderConfig{
			"local": {Recursive: true},
		},
		Light: ThemeConfig{
			Dirs:      []string{picturesDir},
			UploadDir: filepath.Join(configDir, "saved", "light"),
		},
		Dark: ThemeConfig{
			Dirs:      []string{picturesDir},
			UploadDir: filepath.Join(configDir, "saved", "dark"),
		},
	}
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = filepath.Join(DefaultConfigDir(), "config.toml")
	}

	path = expandPath(path)

	cfg := DefaultConfig()
	cfg.configPath = path

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg.postProcess()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) postProcess() {
	c.State.Path = expandPath(c.State.Path)

	for name, p := range c.Providers {
		p.Auth = expandEnv(p.Auth)
		if p.Weight == 0 {
			p.Weight = 1
		}
		c.Providers[name] = p
	}

	c.processTheme(&c.Light)
	c.processTheme(&c.Dark)
}

func (c *Config) processTheme(theme *ThemeConfig) {
	theme.UploadDir = expandPath(theme.UploadDir)
	for i, dir := range theme.Dirs {
		theme.Dirs[i] = expandPath(dir)
	}
}

func (c *Config) Validate() error {
	switch c.Theme.Mode {
	case ThemeModeAuto, ThemeModeLight, ThemeModeDark:
	default:
		return fmt.Errorf("invalid theme mode: %s (must be auto, light, or dark)", c.Theme.Mode)
	}

	for name := range c.Providers {
		if name == "local" {
			continue
		}
		if !isValidProvider(name) {
			return fmt.Errorf("unknown provider: %s", name)
		}
	}

	if err := c.validateThemeProviders("light", &c.Light); err != nil {
		return err
	}
	if err := c.validateThemeProviders("dark", &c.Dark); err != nil {
		return err
	}

	return nil
}

func (c *Config) validateThemeProviders(themeName string, theme *ThemeConfig) error {
	for _, p := range theme.Providers {
		if _, ok := c.Providers[p]; !ok {
			return fmt.Errorf("%s: unknown provider '%s' (not defined in [providers])", themeName, p)
		}
	}

	hasRemote := false
	hasQueryBasedRemote := false

	if len(theme.Providers) == 0 {
		for name := range c.Providers {
			if name != "local" {
				hasRemote = true
				if name != "bing" {
					hasQueryBasedRemote = true
				}
			}
		}
	} else {
		for _, p := range theme.Providers {
			if p != "local" {
				hasRemote = true
				if p != "bing" {
					hasQueryBasedRemote = true
				}
			}
		}
	}

	if hasRemote {
		if theme.UploadDir == "" {
			return fmt.Errorf("%s: upload-dir is required when using remote providers", themeName)
		}
	}

	if hasQueryBasedRemote {
		if len(theme.Queries) == 0 {
			return fmt.Errorf("%s: queries are required when using remote providers", themeName)
		}
	}

	return nil
}

func isValidProvider(name string) bool {
	switch ProviderType(name) {
	case ProviderUnsplash, ProviderWallhaven, ProviderWallhalla, ProviderBing, ProviderLocal:
		return true
	}
	return false
}

func (c *Config) GetThemeConfig(theme ThemeMode) *ThemeConfig {
	switch theme {
	case ThemeModeLight:
		return &c.Light
	case ThemeModeDark:
		return &c.Dark
	default:
		return &c.Light
	}
}

func (c *Config) GetUploadDir(theme ThemeMode) string {
	return c.GetThemeConfig(theme).UploadDir
}

func (c *Config) GetLocalDirs(theme ThemeMode) []string {
	return c.GetThemeConfig(theme).Dirs
}

func (c *Config) GetQueries(theme ThemeMode) []string {
	return c.GetThemeConfig(theme).Queries
}

func (c *Config) GetRemoteProviders(theme ThemeMode) map[string]ProviderConfig {
	themeConfig := c.GetThemeConfig(theme)
	result := make(map[string]ProviderConfig)

	allowedProviders := themeConfig.Providers
	if len(allowedProviders) == 0 {
		for name, p := range c.Providers {
			if name != "local" {
				result[name] = p
			}
		}
		return result
	}

	for _, name := range allowedProviders {
		if name == "local" {
			continue
		}
		if p, ok := c.Providers[name]; ok {
			result[name] = p
		}
	}
	return result
}

func (c *Config) GetLocalConfig() ProviderConfig {
	if p, ok := c.Providers["local"]; ok {
		return p
	}
	return ProviderConfig{Recursive: true}
}

func (c *Config) IsLocalEnabled(theme ThemeMode) bool {
	return len(c.GetLocalDirs(theme)) > 0
}

func (c *Config) ConfigPath() string {
	return c.configPath
}

func (c *Config) Save(path string) error {
	if path == "" {
		path = c.configPath
	}
	if path == "" {
		path = filepath.Join(DefaultConfigDir(), "config.toml")
	}

	path = expandPath(path)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	return nil
}

func (c *Config) EnsureDirectories() error {
	dirs := []string{
		filepath.Dir(c.State.Path),
		c.Light.UploadDir,
		c.Dark.UploadDir,
		GetTempDir(),
	}

	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

func GetTempDir() string {
	return filepath.Join(os.TempDir(), "wallboy")
}

func expandPath(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func expandEnv(s string) string {
	if s == "" {
		return ""
	}

	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		inner := s[2 : len(s)-1]

		if idx := strings.Index(inner, ":-"); idx != -1 {
			varName := inner[:idx]
			defaultVal := inner[idx+2:]
			if val := os.Getenv(varName); val != "" {
				return val
			}
			return defaultVal
		}

		return os.Getenv(inner)
	}

	if strings.HasPrefix(s, "$") && !strings.Contains(s, " ") {
		return os.Getenv(s[1:])
	}

	return s
}
