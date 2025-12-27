// Package config handles configuration loading, parsing and validation for wallboy.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// ThemeMode represents the theme selection mode.
type ThemeMode string

const (
	ThemeModeAuto  ThemeMode = "auto"
	ThemeModeLight ThemeMode = "light"
	ThemeModeDark  ThemeMode = "dark"
)

// DatasourceType represents the type of datasource.
type DatasourceType string

const (
	DatasourceTypeLocal  DatasourceType = "local"
	DatasourceTypeRemote DatasourceType = "remote"
)

// ProviderType represents the type of remote provider.
type ProviderType string

const (
	ProviderUnsplash  ProviderType = "unsplash"
	ProviderWallhaven ProviderType = "wallhaven"
	ProviderGeneric   ProviderType = "generic"
)

// Datasource represents a single image source configuration.
type Datasource struct {
	ID        string         `toml:"id"`
	Type      DatasourceType `toml:"type"`
	Dir       string         `toml:"dir"`
	Recursive bool           `toml:"recursive"`
	Provider  ProviderType   `toml:"provider"`
	Auth      string         `toml:"auth"`
	Queries   []string       `toml:"queries"`
	URL       string         `toml:"url"` // For generic provider
}

// ThemeConfig represents theme-specific configuration.
type ThemeConfig struct {
	UploadDir   string       `toml:"upload-dir"`
	Datasources []Datasource `toml:"datasource"`
}

// StateConfig represents state file configuration.
type StateConfig struct {
	Path string `toml:"path"`
}

// ThemeSettings represents theme mode settings.
type ThemeSettings struct {
	Mode ThemeMode `toml:"mode"`
}

// Config represents the complete configuration.
type Config struct {
	UploadDir string        `toml:"upload-dir"`
	State     StateConfig   `toml:"state"`
	Theme     ThemeSettings `toml:"theme"`
	Light     ThemeConfig   `toml:"light"`
	Dark      ThemeConfig   `toml:"dark"`

	// Runtime fields (not from TOML)
	configPath string
}

// DefaultConfigDir returns the default configuration directory for macOS.
func DefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "wallboy")
}

// DefaultConfig returns a new Config with default values.
func DefaultConfig() *Config {
	configDir := DefaultConfigDir()
	home, _ := os.UserHomeDir()
	picturesDir := filepath.Join(home, "Pictures")

	return &Config{
		UploadDir: filepath.Join(configDir, "saved"),
		State: StateConfig{
			Path: filepath.Join(configDir, "state.json"),
		},
		Theme: ThemeSettings{
			Mode: ThemeModeAuto,
		},
		Light: ThemeConfig{
			UploadDir: filepath.Join(configDir, "saved", "light"),
			Datasources: []Datasource{
				{
					Type:      DatasourceTypeLocal,
					Dir:       picturesDir,
					Recursive: true,
				},
			},
		},
		Dark: ThemeConfig{
			UploadDir: filepath.Join(configDir, "saved", "dark"),
			Datasources: []Datasource{
				{
					Type:      DatasourceTypeLocal,
					Dir:       picturesDir,
					Recursive: true,
				},
			},
		},
	}
}

// Load loads configuration from the specified path or default location.
func Load(path string) (*Config, error) {
	if path == "" {
		path = filepath.Join(DefaultConfigDir(), "config.toml")
	}

	// Expand ~ in path
	path = expandPath(path)

	cfg := DefaultConfig()
	cfg.configPath = path

	// Check if config file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Return default config if file doesn't exist
		return cfg, nil
	}

	// Parse TOML file
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Post-process configuration
	cfg.postProcess()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// postProcess expands paths and generates IDs.
func (c *Config) postProcess() {
	// Expand paths
	c.UploadDir = expandPath(c.UploadDir)
	c.State.Path = expandPath(c.State.Path)
	c.Light.UploadDir = expandPath(c.Light.UploadDir)
	c.Dark.UploadDir = expandPath(c.Dark.UploadDir)

	// Set theme-specific upload dirs if not set
	if c.Light.UploadDir == "" {
		c.Light.UploadDir = filepath.Join(c.UploadDir, "light")
	}
	if c.Dark.UploadDir == "" {
		c.Dark.UploadDir = filepath.Join(c.UploadDir, "dark")
	}

	// Process light datasources
	c.processThemeDatasources("light", &c.Light)
	// Process dark datasources
	c.processThemeDatasources("dark", &c.Dark)
}

// processThemeDatasources expands paths and generates IDs for datasources.
func (c *Config) processThemeDatasources(themeName string, theme *ThemeConfig) {
	localCount := 0
	remoteCount := make(map[ProviderType]int)

	for i := range theme.Datasources {
		ds := &theme.Datasources[i]

		// Expand paths
		ds.Dir = expandPath(ds.Dir)

		// Expand environment variables in auth
		ds.Auth = expandEnv(ds.Auth)

		// Note: Default recursive is set in DefaultConfig
		// TOML doesn't distinguish between false and unset for bools

		// Generate ID if not set
		if ds.ID == "" {
			ds.ID = c.generateDatasourceID(themeName, ds, &localCount, remoteCount)
		}
	}
}

// generateDatasourceID generates a unique ID for a datasource.
func (c *Config) generateDatasourceID(themeName string, ds *Datasource, localCount *int, remoteCount map[ProviderType]int) string {
	switch ds.Type {
	case DatasourceTypeLocal:
		*localCount++
		if *localCount == 1 {
			return fmt.Sprintf("%s-local", themeName)
		}
		return fmt.Sprintf("%s-local-%d", themeName, *localCount)
	case DatasourceTypeRemote:
		remoteCount[ds.Provider]++
		if remoteCount[ds.Provider] == 1 {
			return fmt.Sprintf("%s-%s", themeName, ds.Provider)
		}
		return fmt.Sprintf("%s-%s-%d", themeName, ds.Provider, remoteCount[ds.Provider])
	default:
		return fmt.Sprintf("%s-unknown", themeName)
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	// Validate theme mode
	switch c.Theme.Mode {
	case ThemeModeAuto, ThemeModeLight, ThemeModeDark:
		// Valid
	default:
		return fmt.Errorf("invalid theme mode: %s (must be auto, light, or dark)", c.Theme.Mode)
	}

	// Validate datasources
	if err := c.validateDatasources("light", c.Light.Datasources); err != nil {
		return err
	}
	if err := c.validateDatasources("dark", c.Dark.Datasources); err != nil {
		return err
	}

	return nil
}

// validateDatasources validates a list of datasources.
func (c *Config) validateDatasources(themeName string, datasources []Datasource) error {
	ids := make(map[string]bool)

	for i, ds := range datasources {
		// Check for duplicate IDs
		if ids[ds.ID] {
			return fmt.Errorf("duplicate datasource ID: %s", ds.ID)
		}
		ids[ds.ID] = true

		// Validate based on type
		switch ds.Type {
		case DatasourceTypeLocal:
			if ds.Dir == "" {
				return fmt.Errorf("%s.datasource[%d]: local datasource requires 'dir'", themeName, i)
			}
		case DatasourceTypeRemote:
			if ds.Provider == "" {
				return fmt.Errorf("%s.datasource[%d]: remote datasource requires 'provider'", themeName, i)
			}
			switch ds.Provider {
			case ProviderUnsplash, ProviderWallhaven, ProviderGeneric:
				// Valid
			default:
				return fmt.Errorf("%s.datasource[%d]: unknown provider: %s", themeName, i, ds.Provider)
			}
		default:
			return fmt.Errorf("%s.datasource[%d]: unknown type: %s", themeName, i, ds.Type)
		}
	}

	return nil
}

// GetThemeConfig returns the configuration for the specified theme.
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

// GetUploadDir returns the upload directory for the specified theme.
func (c *Config) GetUploadDir(theme ThemeMode) string {
	themeConfig := c.GetThemeConfig(theme)
	if themeConfig.UploadDir != "" {
		return themeConfig.UploadDir
	}
	return c.UploadDir
}

// GetAllDatasources returns all datasources from both themes.
func (c *Config) GetAllDatasources() []struct {
	Theme      ThemeMode
	Datasource Datasource
} {
	var result []struct {
		Theme      ThemeMode
		Datasource Datasource
	}

	for _, ds := range c.Light.Datasources {
		result = append(result, struct {
			Theme      ThemeMode
			Datasource Datasource
		}{ThemeModeLight, ds})
	}
	for _, ds := range c.Dark.Datasources {
		result = append(result, struct {
			Theme      ThemeMode
			Datasource Datasource
		}{ThemeModeDark, ds})
	}

	return result
}

// FindDatasource finds a datasource by ID across all themes.
func (c *Config) FindDatasource(id string) (*Datasource, ThemeMode, error) {
	for i := range c.Light.Datasources {
		if c.Light.Datasources[i].ID == id {
			return &c.Light.Datasources[i], ThemeModeLight, nil
		}
	}
	for i := range c.Dark.Datasources {
		if c.Dark.Datasources[i].ID == id {
			return &c.Dark.Datasources[i], ThemeModeDark, nil
		}
	}
	return nil, "", fmt.Errorf("datasource not found: %s", id)
}

// ConfigPath returns the path to the config file.
func (c *Config) ConfigPath() string {
	return c.configPath
}

// Save writes the configuration to a file.
func (c *Config) Save(path string) error {
	if path == "" {
		path = c.configPath
	}
	if path == "" {
		path = filepath.Join(DefaultConfigDir(), "config.toml")
	}

	path = expandPath(path)

	// Ensure directory exists
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

// WriteDefaultConfig writes the default configuration to a file.
func WriteDefaultConfig(path string) error {
	cfg := DefaultConfig()
	return cfg.Save(path)
}

// expandPath expands ~ to home directory.
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

// expandEnv expands environment variables in a string.
// Supports formats: ${VAR}, $VAR, ${VAR:-default}
func expandEnv(s string) string {
	if s == "" {
		return ""
	}

	// Check for ${VAR} or ${VAR:-default} format
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		inner := s[2 : len(s)-1]

		// Check for default value syntax: ${VAR:-default}
		if idx := strings.Index(inner, ":-"); idx != -1 {
			varName := inner[:idx]
			defaultVal := inner[idx+2:]
			if val := os.Getenv(varName); val != "" {
				return val
			}
			return defaultVal
		}

		// Simple ${VAR} format
		return os.Getenv(inner)
	}

	// Check for $VAR format (must be entire string)
	if strings.HasPrefix(s, "$") && !strings.Contains(s, " ") {
		varName := s[1:]
		if val := os.Getenv(varName); val != "" {
			return val
		}
		return ""
	}

	// Return as-is if no env var pattern
	return s
}

// EnsureDirectories creates all necessary directories.
func (c *Config) EnsureDirectories() error {
	dirs := []string{
		filepath.Dir(c.State.Path),
		c.UploadDir,
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

// GetTempDir returns the system temp directory for wallboy.
func GetTempDir() string {
	return filepath.Join(os.TempDir(), "wallboy")
}
