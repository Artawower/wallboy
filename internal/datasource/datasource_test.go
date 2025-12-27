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

func TestNewLocalSource(t *testing.T) {
	cfg := config.Datasource{
		ID:        "test-local",
		Type:      config.DatasourceTypeLocal,
		Dir:       "/tmp/wallpapers",
		Recursive: true,
	}

	source := NewLocalSource(cfg, "light")

	assert.Equal(t, "test-local", source.ID())
	assert.Equal(t, config.DatasourceTypeLocal, source.Type())
	assert.Equal(t, "light", source.Theme())
	assert.Equal(t, "/tmp/wallpapers", source.Description())
}

func TestLocalSource_ListImages(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test image files
	files := []string{
		"image1.jpg",
		"image2.png",
		"image3.jpeg",
		"image4.webp",
		"not_image.txt",
		"readme.md",
	}
	for _, f := range files {
		path := filepath.Join(tmpDir, f)
		err := os.WriteFile(path, []byte("test"), 0644)
		require.NoError(t, err)
	}

	// Create subdirectory with images
	subDir := filepath.Join(tmpDir, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "sub_image.jpg"), []byte("test"), 0644))

	t.Run("recursive", func(t *testing.T) {
		cfg := config.Datasource{
			ID:        "test-local",
			Dir:       tmpDir,
			Recursive: true,
		}
		source := NewLocalSource(cfg, "light")

		images, err := source.ListImages(context.Background())
		require.NoError(t, err)

		// Should find 4 images in root + 1 in subdir = 5
		assert.Len(t, images, 5)

		// Check that all are valid images
		for _, img := range images {
			assert.True(t, img.IsLocal)
			assert.Equal(t, "test-local", img.SourceID)
			assert.Equal(t, "light", img.Theme)

			ext := filepath.Ext(img.Path)
			assert.True(t, SupportedExtensions[ext], "unexpected extension: %s", ext)
		}
	})

	t.Run("non-recursive", func(t *testing.T) {
		cfg := config.Datasource{
			ID:        "test-local",
			Dir:       tmpDir,
			Recursive: false,
		}
		source := NewLocalSource(cfg, "dark")

		images, err := source.ListImages(context.Background())
		require.NoError(t, err)

		// Should only find 4 images in root, not subdir
		assert.Len(t, images, 4)
	})

	t.Run("non-existent directory", func(t *testing.T) {
		cfg := config.Datasource{
			ID:  "test-local",
			Dir: "/nonexistent/path",
		}
		source := NewLocalSource(cfg, "light")

		images, err := source.ListImages(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
		assert.Nil(t, images)
	})

	t.Run("context cancellation", func(t *testing.T) {
		cfg := config.Datasource{
			ID:        "test-local",
			Dir:       tmpDir,
			Recursive: true,
		}
		source := NewLocalSource(cfg, "light")

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := source.ListImages(ctx)
		require.Error(t, err)
	})
}

func TestLocalSource_Sync(t *testing.T) {
	cfg := config.Datasource{ID: "test-local", Dir: "/tmp"}
	source := NewLocalSource(cfg, "light")

	// Sync should be a no-op for local sources
	err := source.Sync(context.Background(), nil)
	assert.NoError(t, err)
}

func TestNewManager(t *testing.T) {
	m := NewManager("/upload", "/temp")

	assert.Equal(t, "/upload", m.UploadDir())
	assert.Equal(t, "/temp", m.TempDir())
	assert.Empty(t, m.GetSources())
}

func TestManager_AddSource(t *testing.T) {
	m := NewManager("/upload", "/temp")

	cfg := config.Datasource{ID: "test-local", Dir: "/tmp"}
	source := NewLocalSource(cfg, "light")

	m.AddSource(source)

	sources := m.GetSources()
	require.Len(t, sources, 1)
	assert.Equal(t, "test-local", sources[0].ID())
}

func TestManager_GetSourceByID(t *testing.T) {
	m := NewManager("/upload", "/temp")

	cfg1 := config.Datasource{ID: "source-1", Dir: "/tmp/1"}
	cfg2 := config.Datasource{ID: "source-2", Dir: "/tmp/2"}

	m.AddSource(NewLocalSource(cfg1, "light"))
	m.AddSource(NewLocalSource(cfg2, "dark"))

	t.Run("found", func(t *testing.T) {
		source, err := m.GetSourceByID("source-1")
		require.NoError(t, err)
		assert.Equal(t, "source-1", source.ID())
	})

	t.Run("not found", func(t *testing.T) {
		source, err := m.GetSourceByID("nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		assert.Nil(t, source)
	})
}

func TestManager_GetSourcesByTheme(t *testing.T) {
	m := NewManager("/upload", "/temp")

	cfg1 := config.Datasource{ID: "light-1", Dir: "/tmp/1"}
	cfg2 := config.Datasource{ID: "light-2", Dir: "/tmp/2"}
	cfg3 := config.Datasource{ID: "dark-1", Dir: "/tmp/3"}

	m.AddSource(NewLocalSource(cfg1, "light"))
	m.AddSource(NewLocalSource(cfg2, "light"))
	m.AddSource(NewLocalSource(cfg3, "dark"))

	lightSources := m.GetSourcesByTheme("light")
	assert.Len(t, lightSources, 2)

	darkSources := m.GetSourcesByTheme("dark")
	assert.Len(t, darkSources, 1)

	otherSources := m.GetSourcesByTheme("other")
	assert.Empty(t, otherSources)
}

func TestManager_ListAllImages(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directories with images
	dir1 := filepath.Join(tmpDir, "dir1")
	dir2 := filepath.Join(tmpDir, "dir2")
	require.NoError(t, os.MkdirAll(dir1, 0755))
	require.NoError(t, os.MkdirAll(dir2, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(dir1, "img1.jpg"), []byte("test"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir1, "img2.png"), []byte("test"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "img3.jpg"), []byte("test"), 0644))

	m := NewManager(filepath.Join(tmpDir, "upload"), filepath.Join(tmpDir, "temp"))

	cfg1 := config.Datasource{ID: "source-1", Dir: dir1}
	cfg2 := config.Datasource{ID: "source-2", Dir: dir2}

	m.AddSource(NewLocalSource(cfg1, "light"))
	m.AddSource(NewLocalSource(cfg2, "light"))

	images, err := m.ListAllImages(context.Background(), "light")
	require.NoError(t, err)
	assert.Len(t, images, 3)
}

func TestManager_ListImagesFromSource(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "img.jpg"), []byte("test"), 0644))

	m := NewManager("/upload", "/temp")
	cfg := config.Datasource{ID: "test-source", Dir: tmpDir}
	m.AddSource(NewLocalSource(cfg, "light"))

	t.Run("existing source", func(t *testing.T) {
		images, err := m.ListImagesFromSource(context.Background(), "test-source")
		require.NoError(t, err)
		assert.Len(t, images, 1)
	})

	t.Run("non-existent source", func(t *testing.T) {
		images, err := m.ListImagesFromSource(context.Background(), "nonexistent")
		require.Error(t, err)
		assert.Nil(t, images)
	})
}

func TestManager_PickRandom(t *testing.T) {
	tmpDir := t.TempDir()

	// Create images
	for i := 0; i < 5; i++ {
		path := filepath.Join(tmpDir, filepath.Base(tmpDir), "img"+string(rune('A'+i))+".jpg")
		dir := filepath.Dir(path)
		os.MkdirAll(dir, 0755)
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "img"+string(rune('A'+i))+".jpg"), []byte("test"), 0644))
	}

	m := NewManager("/upload", "/temp")
	cfg := config.Datasource{ID: "test-source", Dir: tmpDir}
	m.AddSource(NewLocalSource(cfg, "light"))

	t.Run("picks image", func(t *testing.T) {
		img, err := m.PickRandom(context.Background(), "light", nil)
		require.NoError(t, err)
		require.NotNil(t, img)
		assert.Contains(t, img.Path, tmpDir)
	})

	t.Run("excludes history", func(t *testing.T) {
		// Get all images
		images, _ := m.ListAllImages(context.Background(), "light")
		history := make([]string, len(images)-1)
		for i := 0; i < len(images)-1; i++ {
			history[i] = images[i].Path
		}

		// Pick should get the remaining one
		img, err := m.PickRandom(context.Background(), "light", history)
		require.NoError(t, err)
		require.NotNil(t, img)
	})

	t.Run("no sources for theme", func(t *testing.T) {
		img, err := m.PickRandom(context.Background(), "nonexistent-theme", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no datasources available")
		assert.Nil(t, img)
	})
}

func TestManager_PickRandomFromSource(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "img1.jpg"), []byte("test"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "img2.jpg"), []byte("test"), 0644))

	m := NewManager("/upload", "/temp")
	cfg := config.Datasource{ID: "test-source", Dir: tmpDir}
	m.AddSource(NewLocalSource(cfg, "light"))

	t.Run("picks from source", func(t *testing.T) {
		img, err := m.PickRandomFromSource(context.Background(), "test-source", nil)
		require.NoError(t, err)
		require.NotNil(t, img)
		assert.Equal(t, "test-source", img.SourceID)
	})

	t.Run("with history filter", func(t *testing.T) {
		img, err := m.PickRandomFromSource(context.Background(), "test-source", []string{filepath.Join(tmpDir, "img1.jpg")})
		require.NoError(t, err)
		require.NotNil(t, img)
	})

	t.Run("non-existent source", func(t *testing.T) {
		img, err := m.PickRandomFromSource(context.Background(), "nonexistent", nil)
		require.Error(t, err)
		assert.Nil(t, img)
	})

	t.Run("empty directory", func(t *testing.T) {
		emptyDir := t.TempDir()
		m2 := NewManager("/upload", "/temp")
		cfg2 := config.Datasource{ID: "empty-source", Dir: emptyDir}
		m2.AddSource(NewLocalSource(cfg2, "light"))

		img, err := m2.PickRandomFromSource(context.Background(), "empty-source", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no images available")
		assert.Nil(t, img)
	})
}

func TestManager_HasRemoteSources(t *testing.T) {
	m := NewManager("/upload", "/temp")

	cfg := config.Datasource{ID: "local-source", Dir: "/tmp"}
	m.AddSource(NewLocalSource(cfg, "light"))

	assert.False(t, m.HasRemoteSources("light"))
	assert.False(t, m.HasRemoteSources("dark"))
}

func TestManager_GetRemoteSourcesForTheme(t *testing.T) {
	m := NewManager("/upload", "/temp")

	cfg := config.Datasource{ID: "local-source", Dir: "/tmp"}
	m.AddSource(NewLocalSource(cfg, "light"))

	// Should return empty list when no remote sources
	remoteSources := m.GetRemoteSourcesForTheme("light")
	assert.Empty(t, remoteSources)
}

func TestManager_FetchRandomFromRemote(t *testing.T) {
	m := NewManager("/upload", "/temp")

	cfg := config.Datasource{ID: "local-source", Dir: "/tmp"}
	m.AddSource(NewLocalSource(cfg, "light"))

	t.Run("not a remote source", func(t *testing.T) {
		img, err := m.FetchRandomFromRemote(context.Background(), "local-source")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a remote source")
		assert.Nil(t, img)
	})

	t.Run("source not found", func(t *testing.T) {
		img, err := m.FetchRandomFromRemote(context.Background(), "nonexistent")
		require.Error(t, err)
		assert.Nil(t, img)
	})
}

func TestManager_SaveCurrentImage(t *testing.T) {
	m := NewManager("/upload", "/temp")

	cfg := config.Datasource{ID: "local-source", Dir: "/tmp"}
	m.AddSource(NewLocalSource(cfg, "light"))

	t.Run("not a remote source", func(t *testing.T) {
		path, err := m.SaveCurrentImage("local-source", "/tmp/img.jpg")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a remote source")
		assert.Empty(t, path)
	})

	t.Run("source not found", func(t *testing.T) {
		path, err := m.SaveCurrentImage("nonexistent", "/tmp/img.jpg")
		require.Error(t, err)
		assert.Empty(t, path)
	})
}

func TestManager_GetRemoteSource(t *testing.T) {
	m := NewManager("/upload", "/temp")

	cfg := config.Datasource{ID: "local-source", Dir: "/tmp"}
	m.AddSource(NewLocalSource(cfg, "light"))

	t.Run("not a remote source", func(t *testing.T) {
		remote, err := m.GetRemoteSource("local-source")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a remote source")
		assert.Nil(t, remote)
	})

	t.Run("source not found", func(t *testing.T) {
		remote, err := m.GetRemoteSource("nonexistent")
		require.Error(t, err)
		assert.Nil(t, remote)
	})
}

func TestManager_CleanupTemp(t *testing.T) {
	m := NewManager("/upload", "/temp")

	cfg := config.Datasource{ID: "local-source", Dir: "/tmp"}
	m.AddSource(NewLocalSource(cfg, "light"))

	// Should not panic
	m.CleanupTemp()
}

func TestSupportedExtensions(t *testing.T) {
	supported := []string{".jpg", ".jpeg", ".png", ".webp"}
	notSupported := []string{".gif", ".bmp", ".svg", ".txt", ".pdf"}

	for _, ext := range supported {
		assert.True(t, SupportedExtensions[ext], "should support %s", ext)
	}

	for _, ext := range notSupported {
		assert.False(t, SupportedExtensions[ext], "should not support %s", ext)
	}
}
