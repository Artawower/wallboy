package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type UnsplashProvider struct {
	*BaseProvider
	accessKey string
}

func NewUnsplashProvider(auth string) *UnsplashProvider {
	accessKey := strings.TrimPrefix(auth, "Bearer ")

	p := &UnsplashProvider{
		BaseProvider: NewBaseProvider(auth),
		accessKey:    accessKey,
	}
	p.baseURL = "https://api.unsplash.com"
	return p
}

func (p *UnsplashProvider) Name() string {
	return "unsplash"
}

func (p *UnsplashProvider) Search(ctx context.Context, queries []string) ([]ImageMeta, error) {
	if len(queries) == 0 {
		return p.fetchRandom(ctx, DefaultSearchLimit)
	}

	var allResults []ImageMeta
	perQuery := max(DefaultSearchLimit/len(queries), 1)

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
				Full    string `json:"full"`
				Regular string `json:"regular"`
			} `json:"urls"`
			Width  int `json:"width"`
			Height int `json:"height"`
			User   struct {
				Name string `json:"name"`
			} `json:"user"`
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

func (p *UnsplashProvider) Download(ctx context.Context, meta ImageMeta, dest string) (string, error) {
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		dest = filepath.Join(dest, fmt.Sprintf("unsplash_%s.jpg", meta.ID))
	}
	return p.downloadFile(ctx, meta.DownloadURL, dest)
}
