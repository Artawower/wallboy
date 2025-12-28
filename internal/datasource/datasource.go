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
	URL      string
}

// SourceType represents the type of source.
type SourceType string

const (
	SourceTypeLocal  SourceType = "local"
	SourceTypeRemote SourceType = "remote"
)

// LocalSource represents a local directory source.
type LocalSource struct {
	id        string
	dir       string
	recursive bool
	theme     string
}

// NewLocalSource creates a new local source.
func NewLocalSource(id, dir, theme string, recursive bool) *LocalSource {
	return &LocalSource{
		id:        id,
		dir:       dir,
		recursive: recursive,
		theme:     theme,
	}
}

func (s *LocalSource) ID() string          { return s.id }
func (s *LocalSource) Type() SourceType    { return SourceTypeLocal }
func (s *LocalSource) Theme() string       { return s.theme }
func (s *LocalSource) Description() string { return s.dir }

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

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if info.IsDir() {
			if !s.recursive && path != s.dir {
				return filepath.SkipDir
			}
			return nil
		}

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

// Manager manages image sources and selection.
type Manager struct {
	localSources  []*LocalSource
	remoteSources []*RemoteSource
	uploadDir     string
	tempDir       string
	rng           *rand.Rand
}

// NewManager creates a new datasource manager.
func NewManager(uploadDir, tempDir string) *Manager {
	return &Manager{
		uploadDir: uploadDir,
		tempDir:   tempDir,
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (m *Manager) TempDir() string   { return m.tempDir }
func (m *Manager) UploadDir() string { return m.uploadDir }

// AddLocalSource adds a local source.
func (m *Manager) AddLocalSource(source *LocalSource) {
	m.localSources = append(m.localSources, source)
}

// AddRemoteSource adds a remote source.
func (m *Manager) AddRemoteSource(source *RemoteSource) {
	m.remoteSources = append(m.remoteSources, source)
}

// GetLocalSources returns all local sources for a theme.
func (m *Manager) GetLocalSources(theme string) []*LocalSource {
	var result []*LocalSource
	for _, s := range m.localSources {
		if s.theme == theme {
			result = append(result, s)
		}
	}
	return result
}

// GetRemoteSources returns all remote sources for a theme.
func (m *Manager) GetRemoteSources(theme string) []*RemoteSource {
	var result []*RemoteSource
	for _, s := range m.remoteSources {
		if s.theme == theme {
			result = append(result, s)
		}
	}
	return result
}

// GetLocalSourceByID returns a local source by ID.
func (m *Manager) GetLocalSourceByID(id string) (*LocalSource, error) {
	for _, s := range m.localSources {
		if s.id == id {
			return s, nil
		}
	}
	return nil, fmt.Errorf("local source not found: %s", id)
}

// GetRemoteSourceByID returns a remote source by ID.
func (m *Manager) GetRemoteSourceByID(id string) (*RemoteSource, error) {
	for _, s := range m.remoteSources {
		if s.id == id {
			return s, nil
		}
	}
	return nil, fmt.Errorf("remote source not found: %s", id)
}

// HasLocalSources returns true if there are local sources for the theme.
func (m *Manager) HasLocalSources(theme string) bool {
	return len(m.GetLocalSources(theme)) > 0
}

// HasRemoteSources returns true if there are remote sources for the theme.
func (m *Manager) HasRemoteSources(theme string) bool {
	return len(m.GetRemoteSources(theme)) > 0
}

// PickRandomLocal picks a random image from local sources.
func (m *Manager) PickRandomLocal(ctx context.Context, theme string, excludeHistory []string) (*Image, error) {
	sources := m.GetLocalSources(theme)
	if len(sources) == 0 {
		return nil, fmt.Errorf("no local sources for theme: %s", theme)
	}

	historySet := make(map[string]bool)
	for _, h := range excludeHistory {
		historySet[h] = true
	}

	type sourceImages struct {
		source *LocalSource
		images []Image
	}
	var available []sourceImages
	var lastErr error

	for _, source := range sources {
		images, err := source.ListImages(ctx)
		if err != nil {
			lastErr = fmt.Errorf("source %s: %w", source.ID(), err)
			continue
		}
		if len(images) == 0 {
			continue
		}

		var filtered []Image
		for _, img := range images {
			if !historySet[img.Path] {
				filtered = append(filtered, img)
			}
		}

		if len(filtered) == 0 {
			filtered = images
		}

		available = append(available, sourceImages{source: source, images: filtered})
	}

	if len(available) == 0 {
		if lastErr != nil {
			return nil, fmt.Errorf("no images available for theme %s: %w", theme, lastErr)
		}
		return nil, fmt.Errorf("no images available for theme: %s", theme)
	}

	sourceIdx := m.rng.Intn(len(available))
	selected := available[sourceIdx]

	imgIdx := m.rng.Intn(len(selected.images))
	return &selected.images[imgIdx], nil
}

// PickRandomFromLocalSource picks a random image from a specific local source.
func (m *Manager) PickRandomFromLocalSource(ctx context.Context, sourceID string, excludeHistory []string) (*Image, error) {
	source, err := m.GetLocalSourceByID(sourceID)
	if err != nil {
		return nil, err
	}

	images, err := source.ListImages(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("no images in source: %s", sourceID)
	}

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

	if len(filtered) == 0 {
		filtered = images
	}

	idx := m.rng.Intn(len(filtered))
	return &filtered[idx], nil
}

// FetchRandomRemote fetches a random image from remote sources.
func (m *Manager) FetchRandomRemote(ctx context.Context, theme, queryOverride string) (*Image, error) {
	sources := m.GetRemoteSources(theme)
	if len(sources) == 0 {
		return nil, fmt.Errorf("no remote sources for theme: %s", theme)
	}

	// Shuffle for random selection
	shuffled := make([]*RemoteSource, len(sources))
	copy(shuffled, sources)
	m.rng.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	// Try each source until success
	var lastErr error
	for _, source := range shuffled {
		img, err := source.FetchRandom(ctx, queryOverride)
		if err == nil {
			return img, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("failed to fetch from remote: %w", lastErr)
}

// CleanupTemp removes temporary files.
func (m *Manager) CleanupTemp() {
	for _, s := range m.remoteSources {
		_ = s.CleanTemp()
	}
}
