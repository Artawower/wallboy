package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		auth         string
		urls         []string
		wantName     string
		wantNil      bool
	}{
		{
			name:         "unsplash",
			providerType: "unsplash",
			auth:         "test-key",
			wantName:     "unsplash",
		},
		{
			name:         "wallhaven",
			providerType: "wallhaven",
			auth:         "test-key",
			wantName:     "wallhaven",
		},
		{
			name:         "bing",
			providerType: "bing",
			wantName:     "bing",
		},
		{
			name:         "wallhalla",
			providerType: "wallhalla",
			wantName:     "wallhalla",
		},
		{
			name:         "generic",
			providerType: "generic",
			urls:         []string{"http://example.com/img.jpg"},
			wantName:     "generic",
		},
		{
			name:         "unknown provider",
			providerType: "unknown",
			wantNil:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProvider(tt.providerType, tt.auth, tt.urls)

			if tt.wantNil {
				assert.Nil(t, p)
				return
			}

			require.NotNil(t, p)
			assert.Equal(t, tt.wantName, p.Name())
		})
	}
}

func TestNewBaseProvider(t *testing.T) {
	p := NewBaseProvider("test-auth")
	require.NotNil(t, p)
	assert.Equal(t, "test-auth", p.auth)
	assert.NotNil(t, p.client)
}

func TestBaseProvider_downloadFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("successful download", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("image data"))
		}))
		defer server.Close()

		p := NewBaseProvider("")
		dest := filepath.Join(tmpDir, "downloaded.jpg")

		path, err := p.downloadFile(context.Background(), server.URL, dest)
		require.NoError(t, err)
		assert.Equal(t, dest, path)

		// Verify file exists and has content
		data, err := os.ReadFile(dest)
		require.NoError(t, err)
		assert.Equal(t, "image data", string(data))
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		p := NewBaseProvider("")
		dest := filepath.Join(tmpDir, "error.jpg")

		_, err := p.downloadFile(context.Background(), server.URL, dest)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("context cancelled", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		p := NewBaseProvider("")
		dest := filepath.Join(tmpDir, "cancelled.jpg")

		_, err := p.downloadFile(ctx, server.URL, dest)
		require.Error(t, err)
	})

	t.Run("creates parent directory", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("data"))
		}))
		defer server.Close()

		p := NewBaseProvider("")
		dest := filepath.Join(tmpDir, "subdir", "nested", "file.jpg")

		path, err := p.downloadFile(context.Background(), server.URL, dest)
		require.NoError(t, err)
		assert.Equal(t, dest, path)
	})
}

// --- Unsplash Provider Tests ---

func TestNewUnsplashProvider(t *testing.T) {
	t.Run("with plain key", func(t *testing.T) {
		p := NewUnsplashProvider("my-access-key")
		assert.Equal(t, "unsplash", p.Name())
		assert.Equal(t, "my-access-key", p.accessKey)
	})

	t.Run("with Bearer prefix", func(t *testing.T) {
		p := NewUnsplashProvider("Bearer my-access-key")
		assert.Equal(t, "my-access-key", p.accessKey)
	})
}

func TestUnsplashProvider_Search(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/search/photos")
		assert.Contains(t, r.Header.Get("Authorization"), "Client-ID")

		response := map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"id": "abc123",
					"urls": map[string]string{
						"raw":     "https://unsplash.com/raw/abc123",
						"full":    "https://unsplash.com/full/abc123",
						"regular": "https://unsplash.com/regular/abc123",
					},
					"width":  1920,
					"height": 1080,
					"user": map[string]string{
						"name": "Test User",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	p := NewUnsplashProvider("test-key")
	p.baseURL = server.URL

	images, err := p.Search(context.Background(), []string{"nature"})
	require.NoError(t, err)
	require.Len(t, images, 1)

	assert.Equal(t, "abc123", images[0].ID)
	assert.Equal(t, "unsplash", images[0].Source)
	assert.Equal(t, "Test User", images[0].Author)
	assert.Equal(t, 1920, images[0].Width)
}

func TestUnsplashProvider_Search_MultipleQueries(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		response := map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"id": "img" + r.URL.Query().Get("query"),
					"urls": map[string]string{
						"regular": "https://example.com/img.jpg",
						"full":    "https://example.com/img.jpg",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	p := NewUnsplashProvider("test-key")
	p.baseURL = server.URL

	images, err := p.Search(context.Background(), []string{"nature", "landscape"})
	require.NoError(t, err)
	assert.Len(t, images, 2)
	assert.Equal(t, 2, callCount)
}

func TestUnsplashProvider_Search_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	p := NewUnsplashProvider("invalid-key")
	p.baseURL = server.URL

	// Should return empty results, not error (continues on per-query errors)
	images, err := p.Search(context.Background(), []string{"nature"})
	require.NoError(t, err)
	assert.Empty(t, images)
}

func TestUnsplashProvider_Download(t *testing.T) {
	tmpDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("image bytes"))
	}))
	defer server.Close()

	p := NewUnsplashProvider("test-key")

	meta := ImageMeta{
		ID:          "test123",
		DownloadURL: server.URL + "/image.jpg",
	}

	t.Run("to specific file", func(t *testing.T) {
		dest := filepath.Join(tmpDir, "specific.jpg")
		path, err := p.Download(context.Background(), meta, dest)
		require.NoError(t, err)
		assert.Equal(t, dest, path)
	})

	t.Run("to directory", func(t *testing.T) {
		path, err := p.Download(context.Background(), meta, tmpDir)
		require.NoError(t, err)
		assert.Contains(t, path, "unsplash_test123.jpg")
	})
}

// --- Wallhaven Provider Tests ---

func TestNewWallhavenProvider(t *testing.T) {
	t.Run("with plain key", func(t *testing.T) {
		p := NewWallhavenProvider("my-api-key")
		assert.Equal(t, "wallhaven", p.Name())
		assert.Equal(t, "my-api-key", p.apiKey)
	})

	t.Run("with Bearer prefix", func(t *testing.T) {
		p := NewWallhavenProvider("Bearer my-api-key")
		assert.Equal(t, "my-api-key", p.apiKey)
	})
}

func TestWallhavenProvider_Search(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/search")

		response := map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":   "abc123",
					"url":  "https://wallhaven.cc/w/abc123",
					"path": "https://w.wallhaven.cc/full/ab/wallhaven-abc123.jpg",
					"thumbs": map[string]string{
						"large": "https://th.wallhaven.cc/lg/ab/abc123.jpg",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	p := NewWallhavenProvider("test-key")
	p.baseURL = server.URL

	images, err := p.Search(context.Background(), []string{"nature"})
	require.NoError(t, err)
	require.Len(t, images, 1)

	assert.Equal(t, "abc123", images[0].ID)
	assert.Equal(t, "wallhaven", images[0].Source)
}

func TestWallhavenProvider_Search_NoAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should not have apikey in query
		assert.NotContains(t, r.URL.RawQuery, "apikey=")
		response := map[string]interface{}{
			"data": []map[string]interface{}{},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	p := NewWallhavenProvider("") // Empty API key
	p.baseURL = server.URL

	_, err := p.Search(context.Background(), []string{"nature"})
	require.NoError(t, err)
}

func TestWallhavenProvider_Download(t *testing.T) {
	tmpDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("image bytes"))
	}))
	defer server.Close()

	p := NewWallhavenProvider("test-key")

	meta := ImageMeta{
		ID:          "test123",
		DownloadURL: server.URL + "/image.png",
	}

	t.Run("to directory uses extension from URL", func(t *testing.T) {
		path, err := p.Download(context.Background(), meta, tmpDir)
		require.NoError(t, err)
		assert.Contains(t, path, "wallhaven_test123.png")
	})

	t.Run("to directory with no extension defaults to jpg", func(t *testing.T) {
		meta.DownloadURL = server.URL + "/image"
		path, err := p.Download(context.Background(), meta, tmpDir)
		require.NoError(t, err)
		assert.Contains(t, path, ".jpg")
	})
}

// --- Bing Provider Tests ---

func TestNewBingProvider(t *testing.T) {
	p := NewBingProvider()
	assert.Equal(t, "bing", p.Name())
	assert.Equal(t, "https://bing.biturl.top", p.baseURL)
}

func TestBingProvider_Search(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check query parameters
		assert.Contains(t, r.URL.RawQuery, "format=json")
		assert.Contains(t, r.URL.RawQuery, "resolution=UHD")

		response := map[string]interface{}{
			"start_date":     "20241228",
			"end_date":       "20241229",
			"url":            "https://www.bing.com/th?id=OHR.TestImage_EN-US123.jpg&rf=LaDigue_UHD.jpg",
			"copyright":      "Test Image (© Test Photographer)",
			"copyright_link": "https://www.bing.com/search?q=test",
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	p := NewBingProvider()
	p.baseURL = server.URL

	// Bing ignores queries, but we call Search with queries anyway
	images, err := p.Search(context.Background(), []string{"ignored query"})
	require.NoError(t, err)
	// Should return multiple images (from indices 0-7 + random)
	assert.NotEmpty(t, images)

	// Check first image
	assert.Equal(t, "20241228", images[0].ID)
	assert.Contains(t, images[0].URL, "bing.com")
	assert.Equal(t, "bing", images[0].Source)
	assert.Contains(t, images[0].Author, "Test")
}

func TestBingProvider_Search_EmptyQueries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"start_date": "20241228",
			"url":        "https://www.bing.com/th?id=test.jpg",
			"copyright":  "Test",
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	p := NewBingProvider()
	p.baseURL = server.URL

	// Bing works without queries
	images, err := p.Search(context.Background(), nil)
	require.NoError(t, err)
	assert.NotEmpty(t, images)
}

func TestBingProvider_Search_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	p := NewBingProvider()
	p.baseURL = server.URL

	// Should return empty results on error (continues through indices)
	images, err := p.Search(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, images)
}

func TestBingProvider_Download(t *testing.T) {
	tmpDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("bing image bytes"))
	}))
	defer server.Close()

	p := NewBingProvider()

	meta := ImageMeta{
		ID:          "20241228",
		DownloadURL: server.URL + "/image.jpg",
	}

	t.Run("to directory", func(t *testing.T) {
		path, err := p.Download(context.Background(), meta, tmpDir)
		require.NoError(t, err)
		assert.Contains(t, path, "bing_20241228.jpg")

		// Verify file exists and has content
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "bing image bytes", string(data))
	})

	t.Run("to specific file", func(t *testing.T) {
		dest := filepath.Join(tmpDir, "specific_bing.jpg")
		path, err := p.Download(context.Background(), meta, dest)
		require.NoError(t, err)
		assert.Equal(t, dest, path)
	})

	t.Run("handles long/invalid extensions", func(t *testing.T) {
		meta.DownloadURL = server.URL + "/image.jpg?rf=LaDigue_UHD.jpg&pid=hp"
		path, err := p.Download(context.Background(), meta, tmpDir)
		require.NoError(t, err)
		// Should default to .jpg when extension is too long (query params)
		assert.Contains(t, path, ".jpg")
	})
}

// --- Wallhalla Provider Tests ---

func TestNewWallhallaProvider(t *testing.T) {
	p := NewWallhallaProvider()

	assert.Equal(t, "wallhalla", p.Name())
	assert.NotNil(t, p.idRegex)
}

func TestWallhallaProvider_Search(t *testing.T) {
	// Mock HTML response with wallpaper links
	mockHTML := `
	<html>
	<body>
		<a href="/wallpaper/15">Image 1</a>
		<a href="/wallpaper/33">Image 2</a>
		<a href="/wallpaper/71">Image 3</a>
		<a href="/wallpaper/15">Duplicate</a>
	</body>
	</html>
	`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(mockHTML))
	}))
	defer server.Close()

	p := NewWallhallaProvider()
	p.baseURL = server.URL

	t.Run("parses random page", func(t *testing.T) {
		images, err := p.Search(context.Background(), nil)
		require.NoError(t, err)
		require.Len(t, images, 3) // 3 unique IDs (15 is duplicated)

		assert.Equal(t, "15", images[0].ID)
		assert.Equal(t, server.URL+"/wallpaper/15", images[0].URL)
		assert.Equal(t, server.URL+"/wallpaper/15/variant/original?dl=true", images[0].DownloadURL)
		assert.Equal(t, "wallhalla", images[0].Source)

		assert.Equal(t, "33", images[1].ID)
		assert.Equal(t, "71", images[2].ID)
	})

	t.Run("with query uses search endpoint", func(t *testing.T) {
		var requestedURL string
		searchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestedURL = r.URL.String()
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(mockHTML))
		}))
		defer searchServer.Close()

		sp := NewWallhallaProvider()
		sp.baseURL = searchServer.URL

		_, err := sp.Search(context.Background(), []string{"minimalist"})
		require.NoError(t, err)
		assert.Contains(t, requestedURL, "/search?q=minimalist")
	})
}

func TestWallhallaProvider_Search_NoResults(t *testing.T) {
	mockHTML := `<html><body>No wallpapers here</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(mockHTML))
	}))
	defer server.Close()

	p := NewWallhallaProvider()
	p.baseURL = server.URL

	_, err := p.Search(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no wallpapers found")
}

func TestWallhallaProvider_Download(t *testing.T) {
	tmpDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("wallhalla image data"))
	}))
	defer server.Close()

	p := NewWallhallaProvider()

	meta := ImageMeta{
		ID:          "42",
		DownloadURL: server.URL + "/wallpaper/42/variant/original",
	}

	t.Run("downloads to directory", func(t *testing.T) {
		path, err := p.Download(context.Background(), meta, tmpDir)
		require.NoError(t, err)
		assert.Contains(t, path, "wallhalla_42.jpg")

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "wallhalla image data", string(data))
	})

	t.Run("downloads to specific file", func(t *testing.T) {
		destPath := filepath.Join(tmpDir, "custom.png")
		path, err := p.Download(context.Background(), meta, destPath)
		require.NoError(t, err)
		assert.Equal(t, destPath, path)
	})
}

// --- Generic Provider Tests ---

func TestNewGenericProvider(t *testing.T) {
	urls := []string{"http://example.com/1.jpg", "http://example.com/2.jpg"}
	p := NewGenericProvider("auth", urls)

	assert.Equal(t, "generic", p.Name())
	assert.Len(t, p.urls, 2)
}

func TestGenericProvider_Search(t *testing.T) {
	urls := []string{
		"http://example.com/img1.jpg",
		"http://example.com/img2.png",
	}
	p := NewGenericProvider("", urls)

	images, err := p.Search(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, images, 2)

	assert.Equal(t, "generic_0", images[0].ID)
	assert.Equal(t, urls[0], images[0].URL)
	assert.Equal(t, urls[0], images[0].DownloadURL)
	assert.Equal(t, "generic", images[0].Source)

	assert.Equal(t, "generic_1", images[1].ID)
}

func TestGenericProvider_Download(t *testing.T) {
	tmpDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("image bytes"))
	}))
	defer server.Close()

	p := NewGenericProvider("", nil)

	meta := ImageMeta{
		ID:          "generic_0",
		DownloadURL: server.URL + "/image.webp",
	}

	t.Run("to directory uses extension from URL", func(t *testing.T) {
		path, err := p.Download(context.Background(), meta, tmpDir)
		require.NoError(t, err)
		assert.Contains(t, path, "generic_0.webp")
	})

	t.Run("to directory with no extension defaults to jpg", func(t *testing.T) {
		meta.DownloadURL = server.URL + "/noext"
		path, err := p.Download(context.Background(), meta, tmpDir)
		require.NoError(t, err)
		assert.Contains(t, path, ".jpg")
	})
}

func TestDefaultSearchLimit(t *testing.T) {
	assert.Equal(t, 50, DefaultSearchLimit)
}
