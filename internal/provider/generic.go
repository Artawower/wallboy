package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type GenericProvider struct {
	*BaseProvider
	urls []string
}

func NewGenericProvider(auth string, urls []string) *GenericProvider {
	return &GenericProvider{
		BaseProvider: NewBaseProvider(auth),
		urls:         urls,
	}
}

func (p *GenericProvider) Name() string {
	return "generic"
}

func (p *GenericProvider) Search(ctx context.Context, queries []string) ([]ImageMeta, error) {
	var images []ImageMeta
	for i, u := range p.urls {
		images = append(images, ImageMeta{
			ID:          fmt.Sprintf("generic_%d", i),
			URL:         u,
			DownloadURL: u,
			Source:      "generic",
		})
	}
	return images, nil
}

func (p *GenericProvider) Download(ctx context.Context, meta ImageMeta, dest string) (string, error) {
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		ext := filepath.Ext(meta.DownloadURL)
		if ext == "" {
			ext = ".jpg"
		}
		dest = filepath.Join(dest, fmt.Sprintf("%s%s", meta.ID, ext))
	}
	return p.downloadFile(ctx, meta.DownloadURL, dest)
}
