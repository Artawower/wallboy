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

type WallhavenProvider struct {
	*BaseProvider
	apiKey string
}

func NewWallhavenProvider(auth string) *WallhavenProvider {
	apiKey := strings.TrimPrefix(auth, "Bearer ")

	p := &WallhavenProvider{
		BaseProvider: NewBaseProvider(auth),
		apiKey:       apiKey,
	}
	p.baseURL = "https://wallhaven.cc/api/v1"
	return p
}

func (p *WallhavenProvider) Name() string {
	return "wallhaven"
}

func (p *WallhavenProvider) Search(ctx context.Context, queries []string) ([]ImageMeta, error) {
	if len(queries) == 0 {
		return p.searchQuery(ctx, "", DefaultSearchLimit)
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

func (p *WallhavenProvider) searchQuery(ctx context.Context, query string, limit int) ([]ImageMeta, error) {
	var u string
	if query == "" {
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

func (p *WallhavenProvider) Download(ctx context.Context, meta ImageMeta, dest string) (string, error) {
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		ext := filepath.Ext(meta.DownloadURL)
		if ext == "" {
			ext = ".jpg"
		}
		dest = filepath.Join(dest, fmt.Sprintf("wallhaven_%s%s", meta.ID, ext))
	}
	return p.downloadFile(ctx, meta.DownloadURL, dest)
}
