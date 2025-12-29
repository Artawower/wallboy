package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const DefaultSearchLimit = 50

type ImageMeta struct {
	ID          string
	URL         string
	DownloadURL string
	Width       int
	Height      int
	Author      string
	Source      string
}

type Provider interface {
	Name() string
	Search(ctx context.Context, queries []string) ([]ImageMeta, error)
	Download(ctx context.Context, meta ImageMeta, dest string) (string, error)
}

type BaseProvider struct {
	client  *http.Client
	auth    string
	baseURL string
}

func NewBaseProvider(auth string) *BaseProvider {
	return &BaseProvider{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		auth: auth,
	}
}

func (p *BaseProvider) downloadFile(ctx context.Context, url, dest string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	f, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return dest, nil
}

func NewProvider(providerType string, auth string, urls []string) Provider {
	switch providerType {
	case "unsplash":
		return NewUnsplashProvider(auth)
	case "wallhaven":
		return NewWallhavenProvider(auth)
	case "bing":
		return NewBingProvider()
	case "wallhalla":
		return NewWallhallaProvider()
	case "generic":
		return NewGenericProvider(auth, urls)
	default:
		return nil
	}
}
