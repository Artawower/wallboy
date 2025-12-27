package datasource

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkawower/wallboy/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRemoteSource(t *testing.T) {
	cfg := config.Datasource{
		ID:       "test-remote",
		Type:     config.DatasourceTypeRemote,
		Provider: config.ProviderUnsplash,
		Auth:     "test-key",
		Queries:  []string{"nature", "landscape"},
	}

	source := NewRemoteSource(cfg, "light", "/upload", "/temp")

	assert.Equal(t, "test-remote", source.ID())
	assert.Equal(t, config.DatasourceTypeRemote, source.Type())
	assert.Equal(t, "light", source.Theme())
	assert.Equal(t, "/upload", source.UploadDir())
	assert.Equal(t, "/temp", source.TempDir())
}

func TestRemoteSource_Description(t *testing.T) {
	t.Run("with provider", func(t *testing.T) {
		cfg := config.Datasource{
			ID:       "test-remote",
			Provider: config.ProviderUnsplash,
		}
		source := NewRemoteSource(cfg, "light", "/upload", "/temp")
		assert.Contains(t, source.Description(), "unsplash")
	})
}

func TestRemoteSource_ListImages(t *testing.T) {
	tmpDir := t.TempDir()
	uploadDir := filepath.Join(tmpDir, "upload")

	cfg := config.Datasource{
		ID:       "test-remote",
		Provider: config.ProviderUnsplash,
	}

	t.Run("empty upload dir", func(t *testing.T) {
		source := NewRemoteSource(cfg, "light", uploadDir, filepath.Join(tmpDir, "temp"))
		images, err := source.ListImages(context.Background())
		require.NoError(t, err)
		assert.Empty(t, images)
	})

	t.Run("with saved images", func(t *testing.T) {
		// Create upload directory with images
		require.NoError(t, os.MkdirAll(uploadDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(uploadDir, "saved1.jpg"), []byte("test"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(uploadDir, "saved2.png"), []byte("test"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(uploadDir, "not_image.txt"), []byte("test"), 0644))

		source := NewRemoteSource(cfg, "dark", uploadDir, filepath.Join(tmpDir, "temp"))
		images, err := source.ListImages(context.Background())
		require.NoError(t, err)
		assert.Len(t, images, 2)

		for _, img := range images {
			assert.Equal(t, "test-remote", img.SourceID)
			assert.Equal(t, "dark", img.Theme)
			assert.False(t, img.IsLocal)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		require.NoError(t, os.MkdirAll(uploadDir, 0755))

		source := NewRemoteSource(cfg, "light", uploadDir, filepath.Join(tmpDir, "temp"))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := source.ListImages(ctx)
		require.Error(t, err)
	})
}

func TestRemoteSource_Save(t *testing.T) {
	tmpDir := t.TempDir()
	tempDir := filepath.Join(tmpDir, "temp")
	uploadDir := filepath.Join(tmpDir, "upload")

	require.NoError(t, os.MkdirAll(tempDir, 0755))

	// Create a temp file to save
	tempFile := filepath.Join(tempDir, "temp_image.jpg")
	require.NoError(t, os.WriteFile(tempFile, []byte("image data"), 0644))

	cfg := config.Datasource{
		ID:       "test-remote",
		Provider: config.ProviderUnsplash,
	}
	source := NewRemoteSource(cfg, "light", uploadDir, tempDir)

	t.Run("save creates upload dir and moves file", func(t *testing.T) {
		savedPath, err := source.Save(tempFile)
		require.NoError(t, err)
		assert.Contains(t, savedPath, uploadDir)

		// Check file exists at new location
		_, err = os.Stat(savedPath)
		require.NoError(t, err)

		// Check temp file was removed
		_, err = os.Stat(tempFile)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestRemoteSource_CleanTemp(t *testing.T) {
	tmpDir := t.TempDir()
	tempDir := filepath.Join(tmpDir, "temp")
	require.NoError(t, os.MkdirAll(tempDir, 0755))

	// Create some temp files
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "temp1.jpg"), []byte("test"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "temp2.jpg"), []byte("test"), 0644))

	cfg := config.Datasource{
		ID:       "test-remote",
		Provider: config.ProviderUnsplash,
	}
	source := NewRemoteSource(cfg, "light", filepath.Join(tmpDir, "upload"), tempDir)

	err := source.CleanTemp()
	require.NoError(t, err)

	// Check files were removed
	entries, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestRemoteSource_CleanTemp_NoDir(t *testing.T) {
	cfg := config.Datasource{
		ID:       "test-remote",
		Provider: config.ProviderUnsplash,
	}

	t.Run("empty temp dir path", func(t *testing.T) {
		source := NewRemoteSource(cfg, "light", "/upload", "")
		err := source.CleanTemp()
		assert.NoError(t, err)
	})

	t.Run("non-existent temp dir", func(t *testing.T) {
		source := NewRemoteSource(cfg, "light", "/upload", "/nonexistent/temp")
		err := source.CleanTemp()
		assert.NoError(t, err)
	})
}

func TestRemoteSource_Sync(t *testing.T) {
	cfg := config.Datasource{
		ID:       "test-remote",
		Provider: config.ProviderUnsplash,
	}
	source := NewRemoteSource(cfg, "light", "/upload", "/temp")

	// Sync should be a no-op
	err := source.Sync(context.Background(), nil)
	assert.NoError(t, err)
}

func TestRemoteSource_FetchRandom_NoProvider(t *testing.T) {
	// Create a source with nil provider manually
	source := &RemoteSource{
		id:        "test",
		provider:  nil,
		uploadDir: "/upload",
		tempDir:   "/temp",
		theme:     "light",
	}

	img, err := source.FetchRandom(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no provider")
	assert.Nil(t, img)
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	srcPath := filepath.Join(tmpDir, "source.txt")
	dstPath := filepath.Join(tmpDir, "dest.txt")

	content := []byte("test content")
	require.NoError(t, os.WriteFile(srcPath, content, 0644))

	err := copyFile(srcPath, dstPath)
	require.NoError(t, err)

	// Check content was copied
	data, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestCopyFile_SourceNotExists(t *testing.T) {
	tmpDir := t.TempDir()

	err := copyFile("/nonexistent/file", filepath.Join(tmpDir, "dest.txt"))
	require.Error(t, err)
}
