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
	"regexp"
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
	// If no queries, use random photos endpoint
	if len(queries) == 0 {
		return p.fetchRandom(ctx, DefaultSearchLimit)
	}

	var allResults []ImageMeta

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

// fetchRandom fetches random photos from Unsplash without a query.
func (p *UnsplashProvider) fetchRandom(ctx context.Context, count int) ([]ImageMeta, error) {
	u := fmt.Sprintf("%s/photos/random?count=%d&orientation=landscape", p.baseURL, count)

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

	var results []struct {
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
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	var images []ImageMeta
	for _, r := range results {
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
	// If no queries, fetch random/popular without search term
	if len(queries) == 0 {
		return p.searchQuery(ctx, "", DefaultSearchLimit)
	}

	var allResults []ImageMeta

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
	var u string
	if query == "" {
		// No query - get random wallpapers
		u = fmt.Sprintf("%s/search?categories=111&purity=100&sorting=random", p.baseURL)
	} else {
		u = fmt.Sprintf("%s/search?q=%s&categories=111&purity=100&sorting=relevance&order=desc",
			p.baseURL, url.QueryEscape(query))
	}

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

// BingProvider implements the Bing Daily Wallpaper API.
type BingProvider struct {
	*BaseProvider
}

// NewBingProvider creates a new Bing provider.
func NewBingProvider() *BingProvider {
	p := &BingProvider{
		BaseProvider: NewBaseProvider(""),
	}
	p.baseURL = "https://bing.biturl.top"
	return p
}

// Name returns the provider name.
func (p *BingProvider) Name() string {
	return "bing"
}

// Search fetches Bing daily wallpapers.
// Bing API returns one image per request, so we fetch multiple with different indices.
func (p *BingProvider) Search(ctx context.Context, queries []string) ([]ImageMeta, error) {
	var images []ImageMeta

	// Fetch images from index 0-7 (Bing keeps last 8 days)
	// Also try random index
	indices := []string{"0", "1", "2", "3", "4", "5", "6", "7", "random"}

	for _, idx := range indices {
		meta, err := p.fetchImage(ctx, idx)
		if err != nil {
			continue
		}
		images = append(images, meta)
	}

	return images, nil
}

func (p *BingProvider) fetchImage(ctx context.Context, index string) (ImageMeta, error) {
	u := fmt.Sprintf("%s/?format=json&index=%s&resolution=UHD&mkt=en-US", p.baseURL, index)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return ImageMeta{}, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return ImageMeta{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ImageMeta{}, fmt.Errorf("API returned status: %d", resp.StatusCode)
	}

	var result struct {
		StartDate     string `json:"start_date"`
		EndDate       string `json:"end_date"`
		URL           string `json:"url"`
		Copyright     string `json:"copyright"`
		CopyrightLink string `json:"copyright_link"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ImageMeta{}, err
	}

	return ImageMeta{
		ID:          result.StartDate,
		URL:         result.URL,
		DownloadURL: result.URL,
		Author:      result.Copyright,
		Source:      "bing",
	}, nil
}

// Download downloads an image from Bing.
func (p *BingProvider) Download(ctx context.Context, meta ImageMeta, dest string) (string, error) {
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		ext := filepath.Ext(meta.DownloadURL)
		if ext == "" || len(ext) > 5 {
			ext = ".jpg"
		}
		dest = filepath.Join(dest, fmt.Sprintf("bing_%s%s", meta.ID, ext))
	}
	return p.downloadFile(ctx, meta.DownloadURL, dest)
}

// WallhallaProvider implements the Wallhalla wallpaper site (HTML scraping).
type WallhallaProvider struct {
	*BaseProvider
	idRegex *regexp.Regexp
}

// NewWallhallaProvider creates a new Wallhalla provider.
func NewWallhallaProvider() *WallhallaProvider {
	p := &WallhallaProvider{
		BaseProvider: NewBaseProvider(""),
		idRegex:      regexp.MustCompile(`/wallpaper/(\d+)`),
	}
	p.baseURL = "https://wallhalla.com"
	return p
}

// Name returns the provider name.
func (p *WallhallaProvider) Name() string {
	return "wallhalla"
}

// Search fetches wallpapers from Wallhalla by scraping HTML.
// If queries are provided, it searches; otherwise fetches random.
func (p *WallhallaProvider) Search(ctx context.Context, queries []string) ([]ImageMeta, error) {
	var searchURL string
	if len(queries) == 0 || queries[0] == "" {
		searchURL = p.baseURL + "/random"
	} else {
		// URL encode the query
		searchURL = fmt.Sprintf("%s/search?q=%s", p.baseURL, url.QueryEscape(queries[0]))
	}

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wallhalla returned status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Extract wallpaper IDs from HTML
	matches := p.idRegex.FindAllStringSubmatch(string(body), -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no wallpapers found on wallhalla")
	}

	// Deduplicate IDs
	seen := make(map[string]bool)
	var images []ImageMeta
	for _, match := range matches {
		id := match[1]
		if seen[id] {
			continue
		}
		seen[id] = true

		images = append(images, ImageMeta{
			ID:          id,
			URL:         fmt.Sprintf("%s/wallpaper/%s", p.baseURL, id),
			DownloadURL: fmt.Sprintf("%s/wallpaper/%s/variant/original?dl=true", p.baseURL, id),
			Source:      "wallhalla",
		})

		if len(images) >= DefaultSearchLimit {
			break
		}
	}

	return images, nil
}

// Download downloads an image from Wallhalla.
func (p *WallhallaProvider) Download(ctx context.Context, meta ImageMeta, dest string) (string, error) {
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		dest = filepath.Join(dest, fmt.Sprintf("wallhalla_%s.jpg", meta.ID))
	}

	req, err := http.NewRequestWithContext(ctx, "GET", meta.DownloadURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download from wallhalla: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("wallhalla download failed with status: %d", resp.StatusCode)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	out, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return dest, nil
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
