// Package datasource provides remote source implementation.
package datasource

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/Artawower/wallboy/internal/config"
	"github.com/Artawower/wallboy/internal/provider"
)

// RemoteSource implements Source for remote providers.
type RemoteSource struct {
	id        string
	provider  provider.Provider
	queries   []string
	uploadDir string
	tempDir   string
	theme     string
	rng       *rand.Rand
}

// NewRemoteSource creates a new remote datasource.
func NewRemoteSource(cfg config.Datasource, theme, uploadDir, tempDir string) *RemoteSource {
	p := provider.NewProvider(string(cfg.Provider), cfg.Auth, nil)

	return &RemoteSource{
		id:        cfg.ID,
		provider:  p,
		queries:   cfg.Queries,
		uploadDir: uploadDir,
		tempDir:   tempDir,
		theme:     theme,
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// ID returns the source ID.
func (s *RemoteSource) ID() string {
	return s.id
}

// Type returns the datasource type.
func (s *RemoteSource) Type() config.DatasourceType {
	return config.DatasourceTypeRemote
}

// Theme returns the theme.
func (s *RemoteSource) Theme() string {
	return s.theme
}

// Description returns a human-readable description.
func (s *RemoteSource) Description() string {
	if s.provider != nil {
		return s.provider.Name()
	}
	return "remote"
}

// ListImages returns all saved images from the upload directory.
func (s *RemoteSource) ListImages(ctx context.Context) ([]Image, error) {
	var images []Image

	// Check if upload directory exists
	if _, err := os.Stat(s.uploadDir); os.IsNotExist(err) {
		return images, nil
	}

	// Walk the upload directory
	err := filepath.Walk(s.uploadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if info.IsDir() {
			return nil
		}

		// Check if it's a supported image
		ext := filepath.Ext(path)
		if !SupportedExtensions[ext] {
			return nil
		}

		images = append(images, Image{
			Path:     path,
			SourceID: s.id,
			Theme:    s.theme,
			IsLocal:  false,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list saved images: %w", err)
	}

	return images, nil
}

// FetchRandom fetches a random image from remote and downloads to temp directory.
// Returns the temporary path to the downloaded image.
func (s *RemoteSource) FetchRandom(ctx context.Context) (*Image, error) {
	if s.provider == nil {
		return nil, fmt.Errorf("no provider configured")
	}

	// Ensure temp directory exists
	if err := os.MkdirAll(s.tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Search for images
	metas, err := s.provider.Search(ctx, s.queries)
	if err != nil {
		return nil, fmt.Errorf("failed to search images: %w", err)
	}

	if len(metas) == 0 {
		return nil, fmt.Errorf("no images found for queries: %v", s.queries)
	}

	// Pick random image
	idx := s.rng.Intn(len(metas))
	meta := metas[idx]

	// Download to temp directory
	tempPath := s.getTempPath(meta)

	// Remove old temp file if exists
	os.Remove(tempPath)

	downloadedPath, err := s.provider.Download(ctx, meta, tempPath)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}

	return &Image{
		Path:     downloadedPath,
		SourceID: s.id,
		Theme:    s.theme,
		IsLocal:  false,
		URL:      meta.DownloadURL,
	}, nil
}

// Save moves an image from temp to upload directory.
func (s *RemoteSource) Save(tempPath string) (string, error) {
	// Ensure upload directory exists
	if err := os.MkdirAll(s.uploadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create upload directory: %w", err)
	}

	filename := filepath.Base(tempPath)
	destPath := filepath.Join(s.uploadDir, filename)

	// Move file
	if err := os.Rename(tempPath, destPath); err != nil {
		// If rename fails (cross-device), try copy+delete
		if err := copyFile(tempPath, destPath); err != nil {
			return "", fmt.Errorf("failed to save image: %w", err)
		}
		os.Remove(tempPath)
	}

	return destPath, nil
}

// CleanTemp removes all files from temp directory.
func (s *RemoteSource) CleanTemp() error {
	if s.tempDir == "" {
		return nil
	}

	entries, err := os.ReadDir(s.tempDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		os.Remove(filepath.Join(s.tempDir, entry.Name()))
	}

	return nil
}

// getTempPath returns the temp path for an image.
func (s *RemoteSource) getTempPath(meta provider.ImageMeta) string {
	ext := filepath.Ext(meta.DownloadURL)
	if ext == "" || len(ext) > 5 {
		ext = ".jpg"
	}
	return filepath.Join(s.tempDir, fmt.Sprintf("%s_%s%s", s.provider.Name(), meta.ID, ext))
}

// Sync is not used in the new flow - kept for interface compatibility.
func (s *RemoteSource) Sync(ctx context.Context, progress func(current, total int)) error {
	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// UploadDir returns the upload directory path.
func (s *RemoteSource) UploadDir() string {
	return s.uploadDir
}

// TempDir returns the temp directory path.
func (s *RemoteSource) TempDir() string {
	return s.tempDir
}
