package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfigDir(t *testing.T) {
	dir := DefaultConfigDir()

	assert.NotEmpty(t, dir)
	assert.Contains(t, dir, ".config/wallboy")
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	require.NotNil(t, cfg)

	// Check theme settings
	assert.Equal(t, ThemeModeAuto, cfg.Theme.Mode)

	// Check upload dir is set
	assert.NotEmpty(t, cfg.UploadDir)
	assert.Contains(t, cfg.UploadDir, "wallboy")

	// Check state path is set
	assert.NotEmpty(t, cfg.State.Path)
	assert.Contains(t, cfg.State.Path, "state.json")

	// Check both themes have default datasources
	assert.NotEmpty(t, cfg.Light.Datasources)
	assert.NotEmpty(t, cfg.Dark.Datasources)

	// Check default datasource is local type
	assert.Equal(t, DatasourceTypeLocal, cfg.Light.Datasources[0].Type)
	assert.Equal(t, DatasourceTypeLocal, cfg.Dark.Datasources[0].Type)
}

func TestGetTempDir(t *testing.T) {
	dir := GetTempDir()

	assert.NotEmpty(t, dir)
	assert.Contains(t, dir, "wallboy")
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		file        string
		wantErr     bool
		errContains string
		validate    func(t *testing.T, cfg *Config)
	}{
		{
			name:    "valid config",
			file:    "testdata/valid.toml",
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				assert.Equal(t, ThemeModeAuto, cfg.Theme.Mode)
				assert.Equal(t, "/tmp/wallboy/saved", cfg.UploadDir)
				assert.Len(t, cfg.Light.Datasources, 2)
				assert.Len(t, cfg.Dark.Datasources, 2)

				// Check IDs are set
				assert.Equal(t, "light-local", cfg.Light.Datasources[0].ID)
				assert.Equal(t, "light-unsplash", cfg.Light.Datasources[1].ID)
				assert.Equal(t, "dark-local", cfg.Dark.Datasources[0].ID)
				assert.Equal(t, "dark-wallhaven", cfg.Dark.Datasources[1].ID)
			},
		},
		{
			name:    "minimal config",
			file:    "testdata/minimal.toml",
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				assert.Equal(t, ThemeModeDark, cfg.Theme.Mode)
				assert.Len(t, cfg.Dark.Datasources, 1)
				// ID should be auto-generated
				assert.Equal(t, "dark-local", cfg.Dark.Datasources[0].ID)
			},
		},
		{
			name:        "invalid syntax",
			file:        "testdata/invalid_syntax.toml",
			wantErr:     true,
			errContains: "failed to parse",
		},
		{
			name:    "non-existent file returns default",
			file:    "testdata/does_not_exist.toml",
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				// Should return default config
				assert.Equal(t, ThemeModeAuto, cfg.Theme.Mode)
			},
		},
		{
			name:    "empty path uses default location",
			file:    "",
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				assert.NotNil(t, cfg)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(tt.file)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)

			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestLoad_WithEnvVars(t *testing.T) {
	// Set test env var
	os.Setenv("TEST_API_KEY", "my-secret-key")
	defer os.Unsetenv("TEST_API_KEY")

	cfg, err := Load("testdata/with_env.toml")
	require.NoError(t, err)

	// Check that env var was expanded
	require.Len(t, cfg.Dark.Datasources, 1)
	assert.Equal(t, "my-secret-key", cfg.Dark.Datasources[0].Auth)
}

func TestConfig_Validate(t *testing.T) {
	// Test validation via file loading where possible
	t.Run("valid config", func(t *testing.T) {
		cfg, err := Load("testdata/valid.toml")
		require.NoError(t, err)
		require.NotNil(t, cfg)
	})

	t.Run("invalid theme mode", func(t *testing.T) {
		cfg, err := Load("testdata/invalid_theme.toml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid theme mode")
		assert.Nil(t, cfg)
	})

	t.Run("duplicate datasource IDs", func(t *testing.T) {
		cfg, err := Load("testdata/duplicate_ids.toml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate datasource ID")
		assert.Nil(t, cfg)
	})

	t.Run("remote datasource without provider", func(t *testing.T) {
		cfg, err := Load("testdata/remote_no_provider.toml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires 'provider'")
		assert.Nil(t, cfg)
	})

	t.Run("unknown provider", func(t *testing.T) {
		cfg, err := Load("testdata/unknown_provider.toml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown provider")
		assert.Nil(t, cfg)
	})

	// Test validation directly for cases that require constructed configs
	t.Run("local datasource without dir", func(t *testing.T) {
		cfg := &Config{
			Theme: ThemeSettings{Mode: ThemeModeLight},
			Light: ThemeConfig{
				Datasources: []Datasource{
					{ID: "test-local", Type: DatasourceTypeLocal, Dir: ""}, // Missing dir
				},
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires 'dir'")
	})

	t.Run("unknown datasource type", func(t *testing.T) {
		cfg := &Config{
			Theme: ThemeSettings{Mode: ThemeModeLight},
			Light: ThemeConfig{
				Datasources: []Datasource{
					{ID: "test-unknown", Type: DatasourceType("invalid")},
				},
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown type")
	})
}

func TestConfig_GetThemeConfig(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		name     string
		mode     ThemeMode
		expected *ThemeConfig
	}{
		{
			name:     "light theme",
			mode:     ThemeModeLight,
			expected: &cfg.Light,
		},
		{
			name:     "dark theme",
			mode:     ThemeModeDark,
			expected: &cfg.Dark,
		},
		{
			name:     "auto defaults to light",
			mode:     ThemeModeAuto,
			expected: &cfg.Light,
		},
		{
			name:     "unknown defaults to light",
			mode:     ThemeMode("unknown"),
			expected: &cfg.Light,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cfg.GetThemeConfig(tt.mode)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfig_GetUploadDir(t *testing.T) {
	cfg, err := Load("testdata/valid.toml")
	require.NoError(t, err)

	tests := []struct {
		name     string
		mode     ThemeMode
		expected string
	}{
		{
			name:     "light theme has specific upload dir",
			mode:     ThemeModeLight,
			expected: "/tmp/wallboy/saved/light",
		},
		{
			name:     "dark theme has specific upload dir",
			mode:     ThemeModeDark,
			expected: "/tmp/wallboy/saved/dark",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cfg.GetUploadDir(tt.mode)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfig_GetUploadDir_Fallback(t *testing.T) {
	cfg := DefaultConfig()
	cfg.UploadDir = "/global/upload"
	cfg.Light.UploadDir = "" // Clear theme-specific

	result := cfg.GetUploadDir(ThemeModeLight)
	assert.Equal(t, "/global/upload", result)
}

func TestConfig_GetAllDatasources(t *testing.T) {
	cfg, err := Load("testdata/valid.toml")
	require.NoError(t, err)

	all := cfg.GetAllDatasources()

	// 2 light + 2 dark = 4 total
	assert.Len(t, all, 4)

	// Check themes are set correctly
	lightCount := 0
	darkCount := 0
	for _, ds := range all {
		if ds.Theme == ThemeModeLight {
			lightCount++
		} else if ds.Theme == ThemeModeDark {
			darkCount++
		}
	}
	assert.Equal(t, 2, lightCount)
	assert.Equal(t, 2, darkCount)
}

func TestConfig_FindDatasource(t *testing.T) {
	cfg, err := Load("testdata/valid.toml")
	require.NoError(t, err)

	tests := []struct {
		name          string
		id            string
		wantErr       bool
		expectedTheme ThemeMode
	}{
		{
			name:          "find in light theme",
			id:            "light-local",
			wantErr:       false,
			expectedTheme: ThemeModeLight,
		},
		{
			name:          "find in dark theme",
			id:            "dark-wallhaven",
			wantErr:       false,
			expectedTheme: ThemeModeDark,
		},
		{
			name:    "not found",
			id:      "nonexistent",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds, theme, err := cfg.FindDatasource(tt.id)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, ds)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, ds)
			assert.Equal(t, tt.id, ds.ID)
			assert.Equal(t, tt.expectedTheme, theme)
		})
	}
}

func TestConfig_Save(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "config.toml")

	cfg := DefaultConfig()
	cfg.Theme.Mode = ThemeModeDark
	cfg.UploadDir = "/custom/upload"

	// Save should create directories
	err := cfg.Save(configPath)
	require.NoError(t, err)

	// File should exist
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Reload and verify
	loaded, err := Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, ThemeModeDark, loaded.Theme.Mode)
	assert.Equal(t, "/custom/upload", loaded.UploadDir)
}

func TestConfig_Save_EmptyPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.configPath = ""

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Should use provided path
	err := cfg.Save(configPath)
	require.NoError(t, err)

	_, err = os.Stat(configPath)
	require.NoError(t, err)
}

func TestConfig_EnsureDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		UploadDir: filepath.Join(tmpDir, "upload"),
		State: StateConfig{
			Path: filepath.Join(tmpDir, "state", "state.json"),
		},
		Light: ThemeConfig{
			UploadDir: filepath.Join(tmpDir, "upload", "light"),
		},
		Dark: ThemeConfig{
			UploadDir: filepath.Join(tmpDir, "upload", "dark"),
		},
	}

	err := cfg.EnsureDirectories()
	require.NoError(t, err)

	// Check directories were created
	dirs := []string{
		cfg.UploadDir,
		filepath.Dir(cfg.State.Path),
		cfg.Light.UploadDir,
		cfg.Dark.UploadDir,
	}

	for _, dir := range dirs {
		info, err := os.Stat(dir)
		require.NoError(t, err, "directory should exist: %s", dir)
		assert.True(t, info.IsDir())
	}
}

func TestConfig_ConfigPath(t *testing.T) {
	cfg, err := Load("testdata/valid.toml")
	require.NoError(t, err)

	assert.Equal(t, "testdata/valid.toml", cfg.ConfigPath())
}

func TestWriteDefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	err := WriteDefaultConfig(configPath)
	require.NoError(t, err)

	// Should be loadable
	cfg, err := Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, ThemeModeAuto, cfg.Theme.Mode)
}
