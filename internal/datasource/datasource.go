// Package datasource provides interfaces and implementations for image sources.
package datasource

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Artawower/wallboy/internal/config"
)

// SupportedExtensions are the image file extensions we support.
var SupportedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

// Image represents an image from a datasource.
type Image struct {
	Path     string
	SourceID string
	Theme    string
	IsLocal  bool
	URL      string // For remote images not yet downloaded
}

// Source is the interface for image sources.
type Source interface {
	// ID returns the unique identifier for this source.
	ID() string

	// Type returns the type of datasource (local/remote).
	Type() config.DatasourceType

	// Theme returns the theme this source belongs to.
	Theme() string

	// ListImages returns all available images from this source.
	ListImages(ctx context.Context) ([]Image, error)

	// Sync downloads/updates images from remote sources.
	// For local sources, this is a no-op.
	Sync(ctx context.Context, progress func(current, total int)) error

	// Description returns a human-readable description of the source.
	Description() string
}

// LocalSource implements Source for local directories.
type LocalSource struct {
	id        string
	dir       string
	recursive bool
	theme     string
}

// NewLocalSource creates a new local datasource.
func NewLocalSource(cfg config.Datasource, theme string) *LocalSource {
	return &LocalSource{
		id:        cfg.ID,
		dir:       cfg.Dir,
		recursive: cfg.Recursive,
		theme:     theme,
	}
}

// ID returns the source ID.
func (s *LocalSource) ID() string {
	return s.id
}

// Type returns the datasource type.
func (s *LocalSource) Type() config.DatasourceType {
	return config.DatasourceTypeLocal
}

// Theme returns the theme.
func (s *LocalSource) Theme() string {
	return s.theme
}

// Description returns a human-readable description.
func (s *LocalSource) Description() string {
	return s.dir
}

// ListImages returns all images from the local directory.
func (s *LocalSource) ListImages(ctx context.Context) ([]Image, error) {
	var images []Image

	if _, err := os.Stat(s.dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory does not exist: %s", s.dir)
	}

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip directories
		if info.IsDir() {
			// Skip subdirectories if not recursive
			if !s.recursive && path != s.dir {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if it's a supported image
		ext := strings.ToLower(filepath.Ext(path))
		if !SupportedExtensions[ext] {
			return nil
		}

		images = append(images, Image{
			Path:     path,
			SourceID: s.id,
			Theme:    s.theme,
			IsLocal:  true,
		})

		return nil
	}

	if err := filepath.Walk(s.dir, walkFn); err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return images, nil
}

// Sync is a no-op for local sources.
func (s *LocalSource) Sync(ctx context.Context, progress func(current, total int)) error {
	return nil
}

// Manager manages multiple datasources.
type Manager struct {
	sources   []Source
	uploadDir string
	tempDir   string
	rng       *rand.Rand
}

// NewManager creates a new datasource manager.
func NewManager(uploadDir, tempDir string) *Manager {
	return &Manager{
		sources:   []Source{},
		uploadDir: uploadDir,
		tempDir:   tempDir,
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// TempDir returns the temp directory path.
func (m *Manager) TempDir() string {
	return m.tempDir
}

// UploadDir returns the upload directory path.
func (m *Manager) UploadDir() string {
	return m.uploadDir
}

// AddSource adds a source to the manager.
func (m *Manager) AddSource(source Source) {
	m.sources = append(m.sources, source)
}

// GetSources returns all sources.
func (m *Manager) GetSources() []Source {
	return m.sources
}

// GetSourceByID returns a source by ID.
func (m *Manager) GetSourceByID(id string) (Source, error) {
	for _, s := range m.sources {
		if s.ID() == id {
			return s, nil
		}
	}
	return nil, fmt.Errorf("source not found: %s", id)
}

// GetSourcesByTheme returns sources for a specific theme.
func (m *Manager) GetSourcesByTheme(theme string) []Source {
	var result []Source
	for _, s := range m.sources {
		if s.Theme() == theme {
			result = append(result, s)
		}
	}
	return result
}

// ListAllImages returns all images from all sources.
func (m *Manager) ListAllImages(ctx context.Context, theme string) ([]Image, error) {
	var allImages []Image

	sources := m.GetSourcesByTheme(theme)
	for _, source := range sources {
		images, err := source.ListImages(ctx)
		if err != nil {
			// Log error but continue with other sources
			continue
		}
		allImages = append(allImages, images...)
	}

	return allImages, nil
}

// ListImagesFromSource returns images from a specific source.
func (m *Manager) ListImagesFromSource(ctx context.Context, sourceID string) ([]Image, error) {
	source, err := m.GetSourceByID(sourceID)
	if err != nil {
		return nil, err
	}
	return source.ListImages(ctx)
}

// PickRandom picks a random image from available images.
// First randomly selects a datasource, then picks a random image from it.
func (m *Manager) PickRandom(ctx context.Context, theme string, excludeHistory []string) (*Image, error) {
	sources := m.GetSourcesByTheme(theme)
	if len(sources) == 0 {
		return nil, fmt.Errorf("no datasources available for theme: %s", theme)
	}

	// Build history set for filtering
	historySet := make(map[string]bool)
	for _, h := range excludeHistory {
		historySet[h] = true
	}

	// Filter sources that have available images
	type sourceWithImages struct {
		source Source
		images []Image
	}
	var availableSources []sourceWithImages

	for _, source := range sources {
		images, err := source.ListImages(ctx)
		if err != nil || len(images) == 0 {
			continue
		}

		// Filter out history
		var filtered []Image
		for _, img := range images {
			if !historySet[img.Path] {
				filtered = append(filtered, img)
			}
		}

		// If all images in history, use all images from this source
		if len(filtered) == 0 {
			filtered = images
		}

		availableSources = append(availableSources, sourceWithImages{
			source: source,
			images: filtered,
		})
	}

	if len(availableSources) == 0 {
		return nil, fmt.Errorf("no images available for theme: %s", theme)
	}

	// Randomly select a datasource
	sourceIdx := m.rng.Intn(len(availableSources))
	selected := availableSources[sourceIdx]

	// Randomly select an image from the chosen datasource
	imgIdx := m.rng.Intn(len(selected.images))
	return &selected.images[imgIdx], nil
}

// PickRandomFromSource picks a random image from a specific source.
func (m *Manager) PickRandomFromSource(ctx context.Context, sourceID string, excludeHistory []string) (*Image, error) {
	images, err := m.ListImagesFromSource(ctx, sourceID)
	if err != nil {
		return nil, err
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("no images available from source: %s", sourceID)
	}

	// Filter out history if provided
	if len(excludeHistory) > 0 {
		historySet := make(map[string]bool)
		for _, h := range excludeHistory {
			historySet[h] = true
		}

		var filtered []Image
		for _, img := range images {
			if !historySet[img.Path] {
				filtered = append(filtered, img)
			}
		}

		if len(filtered) > 0 {
			images = filtered
		}
	}

	idx := m.rng.Intn(len(images))
	return &images[idx], nil
}

// FetchRandomFromRemote fetches a random image from a remote source.
// Downloads to temp directory.
// If queryOverride is not empty, it will be used instead of configured queries.
func (m *Manager) FetchRandomFromRemote(ctx context.Context, sourceID, queryOverride string) (*Image, error) {
	source, err := m.GetSourceByID(sourceID)
	if err != nil {
		return nil, err
	}

	remote, ok := source.(*RemoteSource)
	if !ok {
		return nil, fmt.Errorf("source %s is not a remote source", sourceID)
	}

	return remote.FetchRandom(ctx, queryOverride)
}

// SaveCurrentImage saves an image from temp to upload directory.
func (m *Manager) SaveCurrentImage(sourceID, tempPath string) (string, error) {
	source, err := m.GetSourceByID(sourceID)
	if err != nil {
		return "", err
	}

	remote, ok := source.(*RemoteSource)
	if !ok {
		return "", fmt.Errorf("source %s is not a remote source", sourceID)
	}

	return remote.Save(tempPath)
}

// GetRemoteSource returns a remote source by ID.
func (m *Manager) GetRemoteSource(sourceID string) (*RemoteSource, error) {
	source, err := m.GetSourceByID(sourceID)
	if err != nil {
		return nil, err
	}

	remote, ok := source.(*RemoteSource)
	if !ok {
		return nil, fmt.Errorf("source %s is not a remote source", sourceID)
	}

	return remote, nil
}

// HasRemoteSources returns true if there are any remote sources for the theme.
func (m *Manager) HasRemoteSources(theme string) bool {
	for _, s := range m.GetSourcesByTheme(theme) {
		if s.Type() == config.DatasourceTypeRemote {
			return true
		}
	}
	return false
}

// GetRemoteSourcesForTheme returns all remote sources for a theme.
func (m *Manager) GetRemoteSourcesForTheme(theme string) []*RemoteSource {
	var result []*RemoteSource
	for _, s := range m.GetSourcesByTheme(theme) {
		if remote, ok := s.(*RemoteSource); ok {
			result = append(result, remote)
		}
	}
	return result
}

// CleanupTemp removes temporary files from all remote sources.
func (m *Manager) CleanupTemp() {
	for _, s := range m.sources {
		if remote, ok := s.(*RemoteSource); ok {
			_ = remote.CleanTemp()
		}
	}
}
