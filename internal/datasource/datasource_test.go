package datasource

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLocalSource(t *testing.T) {
	source := NewLocalSource("test-local", "/tmp/wallpapers", "light", true)

	assert.Equal(t, "test-local", source.ID())
	assert.Equal(t, SourceTypeLocal, source.Type())
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
		source := NewLocalSource("test-local", tmpDir, "light", true)

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
		source := NewLocalSource("test-local", tmpDir, "dark", false)

		images, err := source.ListImages(context.Background())
		require.NoError(t, err)

		// Should only find 4 images in root, not subdir
		assert.Len(t, images, 4)
	})

	t.Run("non-existent directory", func(t *testing.T) {
		source := NewLocalSource("test-local", "/nonexistent/path", "light", false)

		images, err := source.ListImages(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
		assert.Nil(t, images)
	})

	t.Run("context cancellation", func(t *testing.T) {
		source := NewLocalSource("test-local", tmpDir, "light", true)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := source.ListImages(ctx)
		require.Error(t, err)
	})
}

func TestNewManager(t *testing.T) {
	m := NewManager("/upload", "/temp")

	assert.Equal(t, "/upload", m.UploadDir())
	assert.Equal(t, "/temp", m.TempDir())
	assert.Empty(t, m.GetLocalSources("any"))
	assert.Empty(t, m.GetRemoteSources("any"))
}

func TestManager_AddLocalSource(t *testing.T) {
	m := NewManager("/upload", "/temp")

	source := NewLocalSource("test-local", "/tmp", "light", false)
	m.AddLocalSource(source)

	sources := m.GetLocalSources("light")
	require.Len(t, sources, 1)
	assert.Equal(t, "test-local", sources[0].ID())
}

func TestManager_GetLocalSources(t *testing.T) {
	m := NewManager("/upload", "/temp")

	m.AddLocalSource(NewLocalSource("light-1", "/tmp/1", "light", false))
	m.AddLocalSource(NewLocalSource("light-2", "/tmp/2", "light", false))
	m.AddLocalSource(NewLocalSource("dark-1", "/tmp/3", "dark", false))

	lightSources := m.GetLocalSources("light")
	assert.Len(t, lightSources, 2)

	darkSources := m.GetLocalSources("dark")
	assert.Len(t, darkSources, 1)

	otherSources := m.GetLocalSources("other")
	assert.Empty(t, otherSources)
}

func TestManager_HasLocalSources(t *testing.T) {
	m := NewManager("/upload", "/temp")

	source := NewLocalSource("local-source", "/tmp", "light", false)
	m.AddLocalSource(source)

	assert.True(t, m.HasLocalSources("light"))
	assert.False(t, m.HasLocalSources("dark"))
}

func TestManager_HasRemoteSources(t *testing.T) {
	m := NewManager("/upload", "/temp")

	source := NewLocalSource("local-source", "/tmp", "light", false)
	m.AddLocalSource(source)

	assert.False(t, m.HasRemoteSources("light"))
	assert.False(t, m.HasRemoteSources("dark"))
}

func TestManager_PickRandomLocal(t *testing.T) {
	tmpDir := t.TempDir()

	// Create images
	for i := 0; i < 5; i++ {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "img"+string(rune('A'+i))+".jpg"), []byte("test"), 0644))
	}

	m := NewManager("/upload", "/temp")
	m.AddLocalSource(NewLocalSource("test-source", tmpDir, "light", false))

	t.Run("picks image", func(t *testing.T) {
		img, err := m.PickRandomLocal(context.Background(), "light", nil)
		require.NoError(t, err)
		require.NotNil(t, img)
		assert.Contains(t, img.Path, tmpDir)
	})

	t.Run("excludes history", func(t *testing.T) {
		// Get all images from the source
		source := m.GetLocalSources("light")[0]
		images, _ := source.ListImages(context.Background())
		history := make([]string, len(images)-1)
		for i := 0; i < len(images)-1; i++ {
			history[i] = images[i].Path
		}

		// Pick should get the remaining one
		img, err := m.PickRandomLocal(context.Background(), "light", history)
		require.NoError(t, err)
		require.NotNil(t, img)
	})

	t.Run("no sources for theme", func(t *testing.T) {
		img, err := m.PickRandomLocal(context.Background(), "nonexistent-theme", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no local sources")
		assert.Nil(t, img)
	})
}

func TestManager_PickRandomLocal_MultipleSources(t *testing.T) {
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

	m.AddLocalSource(NewLocalSource("source-1", dir1, "light", false))
	m.AddLocalSource(NewLocalSource("source-2", dir2, "light", false))

	// Should be able to pick from multiple sources
	img, err := m.PickRandomLocal(context.Background(), "light", nil)
	require.NoError(t, err)
	require.NotNil(t, img)
}

func TestManager_FetchRandomRemote(t *testing.T) {
	m := NewManager("/upload", "/temp")

	// No remote sources
	t.Run("no remote sources", func(t *testing.T) {
		img, err := m.FetchRandomRemote(context.Background(), "light", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no remote sources")
		assert.Nil(t, img)
	})
}

func TestManager_GetRemoteSourceByID(t *testing.T) {
	m := NewManager("/upload", "/temp")

	t.Run("not found", func(t *testing.T) {
		source, err := m.GetRemoteSourceByID("nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		assert.Nil(t, source)
	})
}

func TestManager_GetRemoteSourceByProvider(t *testing.T) {
	m := NewManager("/upload", "/temp")

	// Add remote sources
	m.AddRemoteSource(NewRemoteSource("dark-bing", "bing", "", "dark", "/upload", "/temp", nil, 1, nil))
	m.AddRemoteSource(NewRemoteSource("dark-wallhaven", "wallhaven", "key", "dark", "/upload", "/temp", nil, 1, nil))
	m.AddRemoteSource(NewRemoteSource("light-unsplash", "unsplash", "key", "light", "/upload", "/temp", nil, 1, nil))

	t.Run("finds bing provider for dark theme", func(t *testing.T) {
		source, err := m.GetRemoteSourceByProvider("dark", "bing")
		require.NoError(t, err)
		require.NotNil(t, source)
		assert.Equal(t, "bing", source.ProviderName())
		assert.Equal(t, "dark-bing", source.ID())
	})

	t.Run("finds wallhaven provider for dark theme", func(t *testing.T) {
		source, err := m.GetRemoteSourceByProvider("dark", "wallhaven")
		require.NoError(t, err)
		require.NotNil(t, source)
		assert.Equal(t, "wallhaven", source.ProviderName())
	})

	t.Run("finds unsplash provider for light theme", func(t *testing.T) {
		source, err := m.GetRemoteSourceByProvider("light", "unsplash")
		require.NoError(t, err)
		require.NotNil(t, source)
		assert.Equal(t, "unsplash", source.ProviderName())
	})

	t.Run("not found - wrong theme", func(t *testing.T) {
		source, err := m.GetRemoteSourceByProvider("light", "bing")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		assert.Nil(t, source)
	})

	t.Run("not found - nonexistent provider", func(t *testing.T) {
		source, err := m.GetRemoteSourceByProvider("dark", "nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		assert.Nil(t, source)
	})
}

func TestManager_GetLocalSourceByID(t *testing.T) {
	m := NewManager("/upload", "/temp")

	m.AddLocalSource(NewLocalSource("light-local-1", "/tmp/light", "light", false))

	t.Run("found", func(t *testing.T) {
		source, err := m.GetLocalSourceByID("light-local-1")
		require.NoError(t, err)
		assert.Equal(t, "light-local-1", source.ID())
	})

	t.Run("not found", func(t *testing.T) {
		source, err := m.GetLocalSourceByID("nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		assert.Nil(t, source)
	})
}

func TestManager_PickRandomFromLocalSource(t *testing.T) {
	tmpDir := t.TempDir()

	// Create images
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "img1.jpg"), []byte("test"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "img2.jpg"), []byte("test"), 0644))

	m := NewManager("/upload", "/temp")
	m.AddLocalSource(NewLocalSource("test-source", tmpDir, "light", false))

	t.Run("picks from specific source", func(t *testing.T) {
		img, err := m.PickRandomFromLocalSource(context.Background(), "test-source", nil)
		require.NoError(t, err)
		require.NotNil(t, img)
		assert.Equal(t, "test-source", img.SourceID)
	})

	t.Run("with history filter", func(t *testing.T) {
		img, err := m.PickRandomFromLocalSource(context.Background(), "test-source", []string{filepath.Join(tmpDir, "img1.jpg")})
		require.NoError(t, err)
		require.NotNil(t, img)
	})

	t.Run("source not found", func(t *testing.T) {
		img, err := m.PickRandomFromLocalSource(context.Background(), "nonexistent", nil)
		require.Error(t, err)
		assert.Nil(t, img)
	})

	t.Run("empty source", func(t *testing.T) {
		emptyDir := t.TempDir()
		m2 := NewManager("/upload", "/temp")
		m2.AddLocalSource(NewLocalSource("empty-source", emptyDir, "light", false))

		img, err := m2.PickRandomFromLocalSource(context.Background(), "empty-source", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no images")
		assert.Nil(t, img)
	})
}

func TestManager_PickRandomLocal_ReturnsError(t *testing.T) {
	m := NewManager("/upload", "/temp")

	// Add source with non-existent directory
	m.AddLocalSource(NewLocalSource("bad-source", "/nonexistent/path", "light", false))

	img, err := m.PickRandomLocal(context.Background(), "light", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
	assert.Nil(t, img)
}

func TestManager_CleanupTemp(t *testing.T) {
	m := NewManager("/upload", "/temp")

	source := NewLocalSource("local-source", "/tmp", "light", false)
	m.AddLocalSource(source)

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

func TestManager_WeightedShuffle(t *testing.T) {
	m := NewManager("/upload", "/temp")

	// Create sources with different weights
	// Source A: weight 10, Source B: weight 1
	// A should be selected first ~91% of the time (10/11)
	sourceA := NewRemoteSource("source-a", "bing", "", "dark", "/upload", "/temp", nil, 10, nil)
	sourceB := NewRemoteSource("source-b", "bing", "", "dark", "/upload", "/temp", nil, 1, nil)

	sources := []*RemoteSource{sourceA, sourceB}

	// Run many iterations and count how often A is first
	iterations := 1000
	aFirstCount := 0

	for i := 0; i < iterations; i++ {
		result := m.weightedShuffle(sources)
		if result[0].ID() == "source-a" {
			aFirstCount++
		}
	}

	// A should be first roughly 91% of the time (10/(10+1))
	// Allow some variance: expect between 85% and 97%
	ratio := float64(aFirstCount) / float64(iterations)
	assert.Greater(t, ratio, 0.85, "source-a with weight 10 should be first most of the time")
	assert.Less(t, ratio, 0.97, "but not always (randomness check)")
}

func TestManager_WeightedShuffle_EqualWeights(t *testing.T) {
	m := NewManager("/upload", "/temp")

	// All equal weights should give roughly equal distribution
	sourceA := NewRemoteSource("source-a", "bing", "", "dark", "/upload", "/temp", nil, 1, nil)
	sourceB := NewRemoteSource("source-b", "bing", "", "dark", "/upload", "/temp", nil, 1, nil)
	sourceC := NewRemoteSource("source-c", "bing", "", "dark", "/upload", "/temp", nil, 1, nil)

	sources := []*RemoteSource{sourceA, sourceB, sourceC}

	iterations := 900
	firstCounts := map[string]int{"source-a": 0, "source-b": 0, "source-c": 0}

	for i := 0; i < iterations; i++ {
		result := m.weightedShuffle(sources)
		firstCounts[result[0].ID()]++
	}

	// Each should be first roughly 33% of the time
	// Allow variance: expect between 25% and 42%
	for id, count := range firstCounts {
		ratio := float64(count) / float64(iterations)
		assert.Greater(t, ratio, 0.25, "%s should be first at least 25%% of time", id)
		assert.Less(t, ratio, 0.42, "%s should be first at most 42%% of time", id)
	}
}

func TestManager_WeightedShuffle_SingleSource(t *testing.T) {
	m := NewManager("/upload", "/temp")

	source := NewRemoteSource("only-one", "bing", "", "dark", "/upload", "/temp", nil, 5, nil)
	sources := []*RemoteSource{source}

	result := m.weightedShuffle(sources)
	require.Len(t, result, 1)
	assert.Equal(t, "only-one", result[0].ID())
}

func TestManager_WeightedShuffle_PreservesAllSources(t *testing.T) {
	m := NewManager("/upload", "/temp")

	sourceA := NewRemoteSource("a", "bing", "", "dark", "/upload", "/temp", nil, 5, nil)
	sourceB := NewRemoteSource("b", "bing", "", "dark", "/upload", "/temp", nil, 3, nil)
	sourceC := NewRemoteSource("c", "bing", "", "dark", "/upload", "/temp", nil, 1, nil)

	sources := []*RemoteSource{sourceA, sourceB, sourceC}
	result := m.weightedShuffle(sources)

	require.Len(t, result, 3)

	// All sources should be present
	ids := map[string]bool{}
	for _, s := range result {
		ids[s.ID()] = true
	}
	assert.True(t, ids["a"])
	assert.True(t, ids["b"])
	assert.True(t, ids["c"])
}
