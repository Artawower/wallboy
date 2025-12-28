package datasource

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/Artawower/wallboy/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider is a test provider that records calls and returns configured responses.
type mockProvider struct {
	name          string
	searchQueries [][]string // records all Search calls
	searchResults []provider.ImageMeta
	searchErr     error
	downloadDest  string
	downloadErr   error
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Search(ctx context.Context, queries []string) ([]provider.ImageMeta, error) {
	m.searchQueries = append(m.searchQueries, queries)
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return m.searchResults, nil
}

func (m *mockProvider) Download(ctx context.Context, meta provider.ImageMeta, dest string) (string, error) {
	if m.downloadErr != nil {
		return "", m.downloadErr
	}
	// Create parent directory if needed
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return "", err
	}
	// Create the file at the exact destination path
	if err := os.WriteFile(dest, []byte("image data"), 0644); err != nil {
		return "", err
	}
	return dest, nil
}

func TestNewRemoteSource(t *testing.T) {
	source := NewRemoteSource(
		"test-remote",
		"unsplash",
		"test-key",
		"light",
		"/upload",
		"/temp",
		[]string{"nature", "landscape"},
	)

	assert.Equal(t, "test-remote", source.ID())
	assert.Equal(t, SourceTypeRemote, source.Type())
	assert.Equal(t, "light", source.Theme())
	assert.Equal(t, "/upload", source.UploadDir())
	assert.Equal(t, "/temp", source.TempDir())
}

func TestRemoteSource_Description(t *testing.T) {
	t.Run("with provider", func(t *testing.T) {
		source := NewRemoteSource(
			"test-remote",
			"unsplash",
			"test-key",
			"light",
			"/upload",
			"/temp",
			nil,
		)
		assert.Contains(t, source.Description(), "unsplash")
	})
}

func TestRemoteSource_ListImages(t *testing.T) {
	tmpDir := t.TempDir()
	uploadDir := filepath.Join(tmpDir, "upload")

	t.Run("empty upload dir", func(t *testing.T) {
		source := NewRemoteSource(
			"test-remote",
			"unsplash",
			"test-key",
			"light",
			uploadDir,
			filepath.Join(tmpDir, "temp"),
			nil,
		)
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

		source := NewRemoteSource(
			"test-remote",
			"unsplash",
			"test-key",
			"dark",
			uploadDir,
			filepath.Join(tmpDir, "temp"),
			nil,
		)
		images, err := source.ListImages(context.Background())
		require.NoError(t, err)
		assert.Len(t, images, 2)

		for _, img := range images {
			assert.Equal(t, "test-remote", img.SourceID)
			assert.Equal(t, "dark", img.Theme)
			assert.False(t, img.IsLocal)
		}
	})

	t.Run("with uppercase extensions", func(t *testing.T) {
		upperDir := filepath.Join(tmpDir, "upper")
		require.NoError(t, os.MkdirAll(upperDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(upperDir, "image1.JPG"), []byte("test"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(upperDir, "image2.PNG"), []byte("test"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(upperDir, "image3.JPEG"), []byte("test"), 0644))

		source := NewRemoteSource(
			"test-remote",
			"unsplash",
			"test-key",
			"light",
			upperDir,
			filepath.Join(tmpDir, "temp"),
			nil,
		)
		images, err := source.ListImages(context.Background())
		require.NoError(t, err)
		assert.Len(t, images, 3, "should find images with uppercase extensions")
	})

	t.Run("context cancellation", func(t *testing.T) {
		require.NoError(t, os.MkdirAll(uploadDir, 0755))

		source := NewRemoteSource(
			"test-remote",
			"unsplash",
			"test-key",
			"light",
			uploadDir,
			filepath.Join(tmpDir, "temp"),
			nil,
		)

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

	source := NewRemoteSource(
		"test-remote",
		"unsplash",
		"test-key",
		"light",
		uploadDir,
		tempDir,
		nil,
	)

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

	source := NewRemoteSource(
		"test-remote",
		"unsplash",
		"test-key",
		"light",
		filepath.Join(tmpDir, "upload"),
		tempDir,
		nil,
	)

	err := source.CleanTemp()
	require.NoError(t, err)

	// Check files were removed
	entries, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestRemoteSource_CleanTemp_NoDir(t *testing.T) {
	t.Run("empty temp dir path", func(t *testing.T) {
		source := NewRemoteSource(
			"test-remote",
			"unsplash",
			"test-key",
			"light",
			"/upload",
			"",
			nil,
		)
		err := source.CleanTemp()
		assert.NoError(t, err)
	})

	t.Run("non-existent temp dir", func(t *testing.T) {
		source := NewRemoteSource(
			"test-remote",
			"unsplash",
			"test-key",
			"light",
			"/upload",
			"/nonexistent/temp",
			nil,
		)
		err := source.CleanTemp()
		assert.NoError(t, err)
	})
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

	img, err := source.FetchRandom(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no provider")
	assert.Nil(t, img)
}

func TestRemoteSource_FetchRandom_WithQueryOverride(t *testing.T) {
	tmpDir := t.TempDir()
	tempDir := filepath.Join(tmpDir, "temp")
	uploadDir := filepath.Join(tmpDir, "upload")

	mock := &mockProvider{
		name: "mock",
		searchResults: []provider.ImageMeta{
			{ID: "img1", DownloadURL: "http://example.com/img1.jpg"},
			{ID: "img2", DownloadURL: "http://example.com/img2.jpg"},
		},
	}

	source := &RemoteSource{
		id:        "test-remote",
		provider:  mock,
		queries:   []string{"nature", "landscape"}, // configured queries
		uploadDir: uploadDir,
		tempDir:   tempDir,
		theme:     "light",
		rng:       rand.New(rand.NewSource(42)), // seeded for reproducibility
	}

	t.Run("uses configured queries when no override", func(t *testing.T) {
		mock.searchQueries = nil // reset

		_, err := source.FetchRandom(context.Background(), "")
		require.NoError(t, err)

		require.Len(t, mock.searchQueries, 1)
		assert.Equal(t, []string{"nature", "landscape"}, mock.searchQueries[0])
	})

	t.Run("uses query override instead of configured queries", func(t *testing.T) {
		mock.searchQueries = nil // reset

		_, err := source.FetchRandom(context.Background(), "mountains sunset")
		require.NoError(t, err)

		require.Len(t, mock.searchQueries, 1)
		assert.Equal(t, []string{"mountains sunset"}, mock.searchQueries[0])
	})

	t.Run("query override replaces multiple configured queries", func(t *testing.T) {
		mock.searchQueries = nil // reset

		_, err := source.FetchRandom(context.Background(), "ocean")
		require.NoError(t, err)

		require.Len(t, mock.searchQueries, 1)
		// Should be single override query, not the configured ["nature", "landscape"]
		assert.Equal(t, []string{"ocean"}, mock.searchQueries[0])
	})
}

func TestRemoteSource_FetchRandom_ReturnsImage(t *testing.T) {
	tmpDir := t.TempDir()
	tempDir := filepath.Join(tmpDir, "temp")
	uploadDir := filepath.Join(tmpDir, "upload")

	mock := &mockProvider{
		name: "mock",
		searchResults: []provider.ImageMeta{
			{ID: "img1", DownloadURL: "http://example.com/img1.jpg"},
		},
	}

	source := &RemoteSource{
		id:        "test-remote",
		provider:  mock,
		queries:   []string{"nature"},
		uploadDir: uploadDir,
		tempDir:   tempDir,
		theme:     "dark",
		rng:       rand.New(rand.NewSource(42)),
	}

	img, err := source.FetchRandom(context.Background(), "")
	require.NoError(t, err)
	require.NotNil(t, img)

	assert.Equal(t, "test-remote", img.SourceID)
	assert.Equal(t, "dark", img.Theme)
	assert.False(t, img.IsLocal)
	assert.Contains(t, img.Path, tempDir)
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
