package datasource

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Artawower/wallboy/internal/provider"
)

type RemoteSource struct {
	id            string
	provider      provider.Provider
	queries       []string
	uploadDir     string
	tempDir       string
	theme         string
	weight        int
	rng           *rand.Rand
	prefetchStore PrefetchStore
	prefetchWg    sync.WaitGroup
}

func NewRemoteSource(id, providerName, auth, theme, uploadDir, tempDir string, queries []string, weight int, prefetchStore PrefetchStore) *RemoteSource {
	p := provider.NewProvider(providerName, auth, nil)

	if weight < 1 {
		weight = 1
	}

	return &RemoteSource{
		id:            id,
		provider:      p,
		queries:       queries,
		uploadDir:     uploadDir,
		tempDir:       tempDir,
		theme:         theme,
		weight:        weight,
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())),
		prefetchStore: prefetchStore,
	}
}

func (s *RemoteSource) ID() string        { return s.id }
func (s *RemoteSource) Type() SourceType  { return SourceTypeRemote }
func (s *RemoteSource) Theme() string     { return s.theme }
func (s *RemoteSource) UploadDir() string { return s.uploadDir }
func (s *RemoteSource) TempDir() string   { return s.tempDir }
func (s *RemoteSource) Weight() int       { return s.weight }

func (s *RemoteSource) queryInList(query string) bool {
	for _, q := range s.queries {
		if q == query {
			return true
		}
	}
	return false
}

func (s *RemoteSource) Description() string {
	if s.provider != nil {
		return s.provider.Name()
	}
	return "remote"
}

func (s *RemoteSource) ProviderName() string {
	if s.provider != nil {
		return s.provider.Name()
	}
	return ""
}

func (s *RemoteSource) ListImages(ctx context.Context) ([]Image, error) {
	var images []Image

	if _, err := os.Stat(s.uploadDir); os.IsNotExist(err) {
		return images, nil
	}

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

		ext := strings.ToLower(filepath.Ext(path))
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

func (s *RemoteSource) FetchRandom(ctx context.Context, queryOverride string) (*Image, error) {
	if s.provider == nil {
		return nil, fmt.Errorf("no provider configured")
	}

	if s.prefetchStore != nil {
		if prefetchPath, prefetchQuery, ok := s.prefetchStore.GetPrefetch(s.id); ok {
			prefetchValid := false
			if queryOverride != "" {
				prefetchValid = (prefetchQuery == queryOverride)
			} else {
				prefetchValid = s.queryInList(prefetchQuery)
			}

			if prefetchValid {
				s.prefetchStore.ClearPrefetch(s.id)
				_ = s.prefetchStore.Save()

				s.prefetchWg.Add(1)
				go func() {
					defer s.prefetchWg.Done()
					s.doPrefetch(context.Background(), queryOverride)
				}()

				return &Image{
					Path:     prefetchPath,
					SourceID: s.id,
					Theme:    s.theme,
					IsLocal:  false,
					Query:    prefetchQuery,
				}, nil
			}
			s.prefetchStore.ClearPrefetch(s.id)
			_ = s.prefetchStore.Save()
		}
	}

	img, err := s.doFetch(ctx, queryOverride)
	if err != nil {
		return nil, err
	}

	if s.prefetchStore != nil {
		s.prefetchWg.Add(1)
		go func() {
			defer s.prefetchWg.Done()
			s.doPrefetch(context.Background(), queryOverride)
		}()
	}

	return img, nil
}

func (s *RemoteSource) doFetch(ctx context.Context, queryOverride string) (*Image, error) {
	if err := os.MkdirAll(s.tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	var query string
	if queryOverride != "" {
		query = queryOverride
	} else if len(s.queries) > 0 {
		query = s.queries[s.rng.Intn(len(s.queries))]
	}

	var queries []string
	if query != "" {
		queries = []string{query}
	}

	metas, err := s.provider.Search(ctx, queries)
	if err != nil {
		return nil, fmt.Errorf("failed to search images: %w", err)
	}

	if len(metas) == 0 {
		return nil, fmt.Errorf("no images found for query: %q", query)
	}

	idx := s.rng.Intn(len(metas))
	meta := metas[idx]

	tempPath := s.getTempPath(meta)
	os.Remove(tempPath)

	downloadedPath, err := s.provider.Download(ctx, meta, tempPath)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}

	return &Image{
		Path:     downloadedPath,
		SourceID: s.id,
		Theme:    s.theme,
		Query:    query,
		IsLocal:  false,
		URL:      meta.DownloadURL,
	}, nil
}

func (s *RemoteSource) WaitPrefetch() {
	s.prefetchWg.Wait()
}

func (s *RemoteSource) doPrefetch(ctx context.Context, queryOverride string) {
	if s.prefetchStore == nil {
		return
	}

	img, err := s.doFetch(ctx, queryOverride)
	if err != nil {
		return
	}

	s.prefetchStore.SetPrefetch(s.id, img.Path, img.Query)
	_ = s.prefetchStore.Save()
}

func (s *RemoteSource) Save(tempPath string) (string, error) {
	if err := os.MkdirAll(s.uploadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create upload directory: %w", err)
	}

	filename := filepath.Base(tempPath)
	destPath := filepath.Join(s.uploadDir, filename)

	if err := os.Rename(tempPath, destPath); err != nil {
		if err := copyFile(tempPath, destPath); err != nil {
			return "", fmt.Errorf("failed to save image: %w", err)
		}
		os.Remove(tempPath)
	}

	return destPath, nil
}

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

func (s *RemoteSource) getTempPath(meta provider.ImageMeta) string {
	ext := filepath.Ext(meta.DownloadURL)
	if ext == "" || len(ext) > 5 {
		ext = ".jpg"
	}
	return filepath.Join(s.tempDir, fmt.Sprintf("%s_%s%s", s.provider.Name(), meta.ID, ext))
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
