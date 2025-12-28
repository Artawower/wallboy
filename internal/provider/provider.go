// Package provider implements remote image providers.
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ImageMeta represents metadata for a remote image.
type ImageMeta struct {
	ID          string
	URL         string
	DownloadURL string
	Width       int
	Height      int
	Author      string
	Source      string
}

// DefaultSearchLimit is the default number of results to fetch from providers.
const DefaultSearchLimit = 50

// Provider is the interface for remote image providers.
type Provider interface {
	// Name returns the provider name.
	Name() string

	// Search searches for images matching the queries.
	Search(ctx context.Context, queries []string) ([]ImageMeta, error)

	// Download downloads an image to the destination.
	Download(ctx context.Context, meta ImageMeta, dest string) (string, error)
}

// BaseProvider provides common functionality for providers.
type BaseProvider struct {
	client  *http.Client
	auth    string
	baseURL string
}

// NewBaseProvider creates a new base provider.
func NewBaseProvider(auth string) *BaseProvider {
	return &BaseProvider{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		auth: auth,
	}
}

// downloadFile downloads a file from URL to destination.
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

	// Ensure directory exists
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

// UnsplashProvider implements the Unsplash API.
type UnsplashProvider struct {
	*BaseProvider
	accessKey string
}

// NewUnsplashProvider creates a new Unsplash provider.
func NewUnsplashProvider(auth string) *UnsplashProvider {
	// Auth can be "Bearer <key>" or just the key
	accessKey := auth
	if strings.HasPrefix(auth, "Bearer ") {
		accessKey = strings.TrimPrefix(auth, "Bearer ")
	}

	p := &UnsplashProvider{
		BaseProvider: NewBaseProvider(auth),
		accessKey:    accessKey,
	}
	p.baseURL = "https://api.unsplash.com"
	return p
}

// Name returns the provider name.
func (p *UnsplashProvider) Name() string {
	return "unsplash"
}

// Search searches for images on Unsplash.
func (p *UnsplashProvider) Search(ctx context.Context, queries []string) ([]ImageMeta, error) {
	var allResults []ImageMeta

	// Default query if none provided
	if len(queries) == 0 {
		queries = []string{"random"}
	}

	perQuery := DefaultSearchLimit / len(queries)
	if perQuery < 1 {
		perQuery = 1
	}

	for _, query := range queries {
		results, err := p.searchQuery(ctx, query, perQuery)
		if err != nil {
			continue
		}
		allResults = append(allResults, results...)
	}

	if len(allResults) > DefaultSearchLimit {
		allResults = allResults[:DefaultSearchLimit]
	}

	return allResults, nil
}

func (p *UnsplashProvider) searchQuery(ctx context.Context, query string, limit int) ([]ImageMeta, error) {
	u := fmt.Sprintf("%s/search/photos?query=%s&per_page=%d&orientation=landscape",
		p.baseURL, url.QueryEscape(query), limit)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Client-ID "+p.accessKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status: %d", resp.StatusCode)
	}

	var result struct {
		Results []struct {
			ID   string `json:"id"`
			URLs struct {
				Raw     string `json:"raw"`
				Full    string `json:"full"`
				Regular string `json:"regular"`
			} `json:"urls"`
			Width  int `json:"width"`
			Height int `json:"height"`
			User   struct {
				Name string `json:"name"`
			} `json:"user"`
			Links struct {
				Download string `json:"download_location"`
			} `json:"links"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var images []ImageMeta
	for _, r := range result.Results {
		images = append(images, ImageMeta{
			ID:          r.ID,
			URL:         r.URLs.Regular,
			DownloadURL: r.URLs.Full,
			Width:       r.Width,
			Height:      r.Height,
			Author:      r.User.Name,
			Source:      "unsplash",
		})
	}

	return images, nil
}

// Download downloads an image from Unsplash.
func (p *UnsplashProvider) Download(ctx context.Context, meta ImageMeta, dest string) (string, error) {
	// Generate filename if dest is a directory
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		dest = filepath.Join(dest, fmt.Sprintf("unsplash_%s.jpg", meta.ID))
	}
	return p.downloadFile(ctx, meta.DownloadURL, dest)
}

// WallhavenProvider implements the Wallhaven API.
type WallhavenProvider struct {
	*BaseProvider
	apiKey string
}

// NewWallhavenProvider creates a new Wallhaven provider.
func NewWallhavenProvider(auth string) *WallhavenProvider {
	apiKey := auth
	if strings.HasPrefix(auth, "Bearer ") {
		apiKey = strings.TrimPrefix(auth, "Bearer ")
	}

	p := &WallhavenProvider{
		BaseProvider: NewBaseProvider(auth),
		apiKey:       apiKey,
	}
	p.baseURL = "https://wallhaven.cc/api/v1"
	return p
}

// Name returns the provider name.
func (p *WallhavenProvider) Name() string {
	return "wallhaven"
}

// Search searches for images on Wallhaven.
func (p *WallhavenProvider) Search(ctx context.Context, queries []string) ([]ImageMeta, error) {
	var allResults []ImageMeta

	// Default query if none provided
	if len(queries) == 0 {
		queries = []string{"random"}
	}

	perQuery := DefaultSearchLimit / len(queries)
	if perQuery < 1 {
		perQuery = 1
	}

	for _, query := range queries {
		results, err := p.searchQuery(ctx, query, perQuery)
		if err != nil {
			continue
		}
		allResults = append(allResults, results...)
	}

	if len(allResults) > DefaultSearchLimit {
		allResults = allResults[:DefaultSearchLimit]
	}

	return allResults, nil
}

func (p *WallhavenProvider) searchQuery(ctx context.Context, query string, limit int) ([]ImageMeta, error) {
	u := fmt.Sprintf("%s/search?q=%s&categories=111&purity=100&sorting=relevance&order=desc",
		p.baseURL, url.QueryEscape(query))

	if p.apiKey != "" {
		u += "&apikey=" + p.apiKey
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status: %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			ID         string `json:"id"`
			URL        string `json:"url"`
			Path       string `json:"path"`
			Resolution string `json:"resolution"`
			Thumbs     struct {
				Large string `json:"large"`
			} `json:"thumbs"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var images []ImageMeta
	for i, r := range result.Data {
		if i >= limit {
			break
		}
		images = append(images, ImageMeta{
			ID:          r.ID,
			URL:         r.Thumbs.Large,
			DownloadURL: r.Path,
			Source:      "wallhaven",
		})
	}

	return images, nil
}

// Download downloads an image from Wallhaven.
func (p *WallhavenProvider) Download(ctx context.Context, meta ImageMeta, dest string) (string, error) {
	// Generate filename if dest is a directory
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		ext := filepath.Ext(meta.DownloadURL)
		if ext == "" {
			ext = ".jpg"
		}
		dest = filepath.Join(dest, fmt.Sprintf("wallhaven_%s%s", meta.ID, ext))
	}
	return p.downloadFile(ctx, meta.DownloadURL, dest)
}

// GenericProvider implements a generic URL-based provider.
type GenericProvider struct {
	*BaseProvider
	urls []string
}

// NewGenericProvider creates a new generic provider.
func NewGenericProvider(auth string, urls []string) *GenericProvider {
	return &GenericProvider{
		BaseProvider: NewBaseProvider(auth),
		urls:         urls,
	}
}

// Name returns the provider name.
func (p *GenericProvider) Name() string {
	return "generic"
}

// Search returns the configured URLs as image metadata.
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

// Download downloads an image from a URL.
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

// NewProvider creates a provider based on type.
func NewProvider(providerType string, auth string, urls []string) Provider {
	switch providerType {
	case "unsplash":
		return NewUnsplashProvider(auth)
	case "wallhaven":
		return NewWallhavenProvider(auth)
	case "generic":
		return NewGenericProvider(auth, urls)
	default:
		return nil
	}
}
