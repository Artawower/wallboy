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

	assert.Equal(t, ThemeModeAuto, cfg.Theme.Mode)
	assert.NotEmpty(t, cfg.State.Path)
	assert.Contains(t, cfg.State.Path, "state.json")

	// Check both themes have default dirs
	assert.NotEmpty(t, cfg.Light.Dirs)
	assert.NotEmpty(t, cfg.Dark.Dirs)
	assert.NotEmpty(t, cfg.Light.UploadDir)
	assert.NotEmpty(t, cfg.Dark.UploadDir)

	// Check local provider is configured by default
	localCfg, ok := cfg.Providers["local"]
	assert.True(t, ok)
	assert.True(t, localCfg.Recursive)
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

				// Check providers
				unsplash, ok := cfg.Providers["unsplash"]
				assert.True(t, ok)
				assert.Equal(t, "test-key", unsplash.Auth)

				wallhaven, ok := cfg.Providers["wallhaven"]
				assert.True(t, ok)
				assert.Equal(t, "test-api-key", wallhaven.Auth)

				// Check themes
				assert.Equal(t, []string{"/tmp/wallboy/pictures/light"}, cfg.Light.Dirs)
				assert.Equal(t, "/tmp/wallboy/saved/light", cfg.Light.UploadDir)
				assert.Equal(t, []string{"nature", "minimal"}, cfg.Light.Queries)

				assert.Equal(t, []string{"/tmp/wallboy/pictures/dark"}, cfg.Dark.Dirs)
				assert.Equal(t, "/tmp/wallboy/saved/dark", cfg.Dark.UploadDir)
				assert.Equal(t, []string{"dark", "space"}, cfg.Dark.Queries)
			},
		},
		{
			name:    "minimal config",
			file:    "testdata/minimal.toml",
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				assert.Equal(t, ThemeModeDark, cfg.Theme.Mode)
				assert.Equal(t, []string{"/tmp/pictures"}, cfg.Dark.Dirs)
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
				assert.Equal(t, ThemeModeAuto, cfg.Theme.Mode)
			},
		},
		// Note: "empty path uses default location" test removed because it depends
		// on user's actual config file which may be in old format or missing.
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
	os.Setenv("TEST_API_KEY", "my-secret-key")
	defer os.Unsetenv("TEST_API_KEY")

	cfg, err := Load("testdata/with_env.toml")
	require.NoError(t, err)

	wallhaven, ok := cfg.Providers["wallhaven"]
	require.True(t, ok)
	assert.Equal(t, "my-secret-key", wallhaven.Auth)
}

func TestConfig_Validate(t *testing.T) {
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

	t.Run("unknown provider", func(t *testing.T) {
		cfg, err := Load("testdata/unknown_provider.toml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown provider")
		assert.Nil(t, cfg)
	})

	t.Run("provider without auth", func(t *testing.T) {
		cfg := &Config{
			Theme: ThemeSettings{Mode: ThemeModeLight},
			Providers: map[string]ProviderConfig{
				"unsplash": {Auth: ""}, // Missing auth
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "auth is required")
	})

	t.Run("theme references unknown provider", func(t *testing.T) {
		cfg := &Config{
			Theme: ThemeSettings{Mode: ThemeModeLight},
			Providers: map[string]ProviderConfig{
				"unsplash": {Auth: "key"},
			},
			Light: ThemeConfig{
				Providers: []string{"wallhaven"}, // Not defined
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown provider 'wallhaven'")
	})

	t.Run("remote provider requires upload-dir", func(t *testing.T) {
		cfg := &Config{
			Theme: ThemeSettings{Mode: ThemeModeLight},
			Providers: map[string]ProviderConfig{
				"unsplash": {Auth: "key"},
			},
			Light: ThemeConfig{
				Queries:   []string{"nature"},
				UploadDir: "", // Missing
			},
			Dark: ThemeConfig{
				Queries:   []string{"dark"},
				UploadDir: "/tmp/dark",
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "upload-dir is required")
	})

	t.Run("remote provider requires queries", func(t *testing.T) {
		cfg := &Config{
			Theme: ThemeSettings{Mode: ThemeModeLight},
			Providers: map[string]ProviderConfig{
				"unsplash": {Auth: "key"},
			},
			Light: ThemeConfig{
				UploadDir: "/tmp/light",
				Queries:   nil, // Missing
			},
			Dark: ThemeConfig{
				Queries:   []string{"dark"},
				UploadDir: "/tmp/dark",
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "queries are required")
	})

	t.Run("local only does not require upload-dir or queries", func(t *testing.T) {
		cfg := &Config{
			Theme: ThemeSettings{Mode: ThemeModeLight},
			Providers: map[string]ProviderConfig{
				"local": {Recursive: true},
			},
			Light: ThemeConfig{
				Dirs: []string{"/tmp/pictures"},
			},
			Dark: ThemeConfig{
				Dirs: []string{"/tmp/pictures"},
			},
		}
		err := cfg.Validate()
		require.NoError(t, err)
	})
}

func TestConfig_GetThemeConfig(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		name     string
		mode     ThemeMode
		expected *ThemeConfig
	}{
		{"light theme", ThemeModeLight, &cfg.Light},
		{"dark theme", ThemeModeDark, &cfg.Dark},
		{"auto defaults to light", ThemeModeAuto, &cfg.Light},
		{"unknown defaults to light", ThemeMode("unknown"), &cfg.Light},
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

	assert.Equal(t, "/tmp/wallboy/saved/light", cfg.GetUploadDir(ThemeModeLight))
	assert.Equal(t, "/tmp/wallboy/saved/dark", cfg.GetUploadDir(ThemeModeDark))
}

func TestConfig_GetLocalDirs(t *testing.T) {
	cfg, err := Load("testdata/valid.toml")
	require.NoError(t, err)

	assert.Equal(t, []string{"/tmp/wallboy/pictures/light"}, cfg.GetLocalDirs(ThemeModeLight))
	assert.Equal(t, []string{"/tmp/wallboy/pictures/dark"}, cfg.GetLocalDirs(ThemeModeDark))
}

func TestConfig_GetQueries(t *testing.T) {
	cfg, err := Load("testdata/valid.toml")
	require.NoError(t, err)

	assert.Equal(t, []string{"nature", "minimal"}, cfg.GetQueries(ThemeModeLight))
	assert.Equal(t, []string{"dark", "space"}, cfg.GetQueries(ThemeModeDark))
}

func TestConfig_GetRemoteProviders(t *testing.T) {
	t.Run("all providers when not restricted", func(t *testing.T) {
		cfg, err := Load("testdata/valid.toml")
		require.NoError(t, err)

		providers := cfg.GetRemoteProviders(ThemeModeLight)
		assert.Len(t, providers, 2)
		assert.Contains(t, providers, "unsplash")
		assert.Contains(t, providers, "wallhaven")
		assert.NotContains(t, providers, "local")
	})

	t.Run("restricted to specific providers", func(t *testing.T) {
		cfg := &Config{
			Providers: map[string]ProviderConfig{
				"unsplash":  {Auth: "key1"},
				"wallhaven": {Auth: "key2"},
			},
			Light: ThemeConfig{
				Providers: []string{"unsplash"}, // Only unsplash
			},
		}

		providers := cfg.GetRemoteProviders(ThemeModeLight)
		assert.Len(t, providers, 1)
		assert.Contains(t, providers, "unsplash")
		assert.NotContains(t, providers, "wallhaven")
	})
}

func TestConfig_GetLocalConfig(t *testing.T) {
	cfg, err := Load("testdata/valid.toml")
	require.NoError(t, err)

	localCfg := cfg.GetLocalConfig()
	assert.True(t, localCfg.Recursive)
}

func TestConfig_IsLocalEnabled(t *testing.T) {
	cfg, err := Load("testdata/valid.toml")
	require.NoError(t, err)

	assert.True(t, cfg.IsLocalEnabled(ThemeModeLight))
	assert.True(t, cfg.IsLocalEnabled(ThemeModeDark))

	cfg.Light.Dirs = nil
	assert.False(t, cfg.IsLocalEnabled(ThemeModeLight))
}

func TestConfig_Save(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "config.toml")

	cfg := DefaultConfig()
	cfg.Theme.Mode = ThemeModeDark

	err := cfg.Save(configPath)
	require.NoError(t, err)

	_, err = os.Stat(configPath)
	require.NoError(t, err)

	loaded, err := Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, ThemeModeDark, loaded.Theme.Mode)
}

func TestConfig_EnsureDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
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

	dirs := []string{
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

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"/absolute/path", "/absolute/path"},
		{"~/test", filepath.Join(home, "test")},
		{"relative", "relative"},
	}

	for _, tt := range tests {
		result := expandPath(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestExpandEnv(t *testing.T) {
	os.Setenv("TEST_VAR", "test_value")
	defer os.Unsetenv("TEST_VAR")

	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"plain text", "plain text"},
		{"${TEST_VAR}", "test_value"},
		{"$TEST_VAR", "test_value"},
		{"${NONEXISTENT}", ""},
		{"${NONEXISTENT:-default}", "default"},
	}

	for _, tt := range tests {
		result := expandEnv(tt.input)
		assert.Equal(t, tt.expected, result, "input: %s", tt.input)
	}
}
