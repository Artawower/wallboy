package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

type BingProvider struct {
	*BaseProvider
}

func NewBingProvider() *BingProvider {
	p := &BingProvider{
		BaseProvider: NewBaseProvider(""),
	}
	p.baseURL = "https://bing.biturl.top"
	return p
}

func (p *BingProvider) Name() string {
	return "bing"
}

func (p *BingProvider) Search(ctx context.Context, queries []string) ([]ImageMeta, error) {
	var images []ImageMeta

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
