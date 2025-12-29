package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

type WallhallaProvider struct {
	*BaseProvider
	idRegex *regexp.Regexp
}

func NewWallhallaProvider() *WallhallaProvider {
	p := &WallhallaProvider{
		BaseProvider: &BaseProvider{
			client: &http.Client{
				Timeout: 60 * time.Second,
			},
			auth: "",
		},
		idRegex: regexp.MustCompile(`/wallpaper/(\d+)`),
	}
	p.baseURL = "https://wallhalla.com"
	return p
}

func (p *WallhallaProvider) Name() string {
	return "wallhalla"
}

func (p *WallhallaProvider) Search(ctx context.Context, queries []string) ([]ImageMeta, error) {
	if len(queries) == 0 || queries[0] == "" {
		return p.fetchPage(ctx, p.baseURL+"/random")
	}

	images, err := p.fetchPage(ctx, fmt.Sprintf("%s/search?q=%s", p.baseURL, url.QueryEscape(queries[0])))
	if err != nil || len(images) == 0 {
		return p.fetchPage(ctx, p.baseURL+"/random")
	}
	return images, nil
}

func (p *WallhallaProvider) fetchPage(ctx context.Context, pageURL string) ([]ImageMeta, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Connection", "keep-alive")

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

	matches := p.idRegex.FindAllStringSubmatch(string(body), -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no wallpapers found on wallhalla")
	}

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
