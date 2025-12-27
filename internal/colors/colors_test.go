package colors

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestImage creates a test image with specified colors
func createTestImage(t *testing.T, path string, width, height int, colors []color.Color) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill image with colors in stripes
	if len(colors) > 0 {
		stripeHeight := height / len(colors)
		for i, c := range colors {
			for y := i * stripeHeight; y < (i+1)*stripeHeight; y++ {
				for x := 0; x < width; x++ {
					img.Set(x, y, c)
				}
			}
		}
	}

	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	err = png.Encode(f, img)
	require.NoError(t, err)
}

func TestColor_Hex(t *testing.T) {
	tests := []struct {
		name     string
		color    Color
		expected string
	}{
		{
			name:     "black",
			color:    Color{R: 0, G: 0, B: 0},
			expected: "#000000",
		},
		{
			name:     "white",
			color:    Color{R: 255, G: 255, B: 255},
			expected: "#ffffff",
		},
		{
			name:     "red",
			color:    Color{R: 255, G: 0, B: 0},
			expected: "#ff0000",
		},
		{
			name:     "green",
			color:    Color{R: 0, G: 255, B: 0},
			expected: "#00ff00",
		},
		{
			name:     "blue",
			color:    Color{R: 0, G: 0, B: 255},
			expected: "#0000ff",
		},
		{
			name:     "custom",
			color:    Color{R: 171, G: 205, B: 239},
			expected: "#abcdef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.color.Hex()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAnalyze(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("single color image", func(t *testing.T) {
		imgPath := filepath.Join(tmpDir, "single_color.png")
		createTestImage(t, imgPath, 100, 100, []color.Color{
			color.RGBA{R: 255, G: 0, B: 0, A: 255}, // Red
		})

		colors, err := Analyze(imgPath, 3)
		require.NoError(t, err)
		require.NotEmpty(t, colors)

		// The dominant color should be close to red
		assert.Greater(t, int(colors[0].R), 200)
		assert.Less(t, int(colors[0].G), 50)
		assert.Less(t, int(colors[0].B), 50)
	})

	t.Run("two color image", func(t *testing.T) {
		imgPath := filepath.Join(tmpDir, "two_colors.png")
		createTestImage(t, imgPath, 100, 100, []color.Color{
			color.RGBA{R: 255, G: 0, B: 0, A: 255}, // Red
			color.RGBA{R: 0, G: 0, B: 255, A: 255}, // Blue
		})

		colors, err := Analyze(imgPath, 3)
		require.NoError(t, err)
		require.Len(t, colors, 3)
	})

	t.Run("non-existent file", func(t *testing.T) {
		colors, err := Analyze("/nonexistent/path.jpg", 3)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to open")
		assert.Nil(t, colors)
	})

	t.Run("invalid image file", func(t *testing.T) {
		invalidPath := filepath.Join(tmpDir, "invalid.png")
		err := os.WriteFile(invalidPath, []byte("not an image"), 0644)
		require.NoError(t, err)

		colors, err := Analyze(invalidPath, 3)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode")
		assert.Nil(t, colors)
	})
}

func TestResizeImage(t *testing.T) {
	t.Run("image smaller than max", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 50, 50))
		resized := resizeImage(img, 200, 200)

		// Should not resize
		bounds := resized.Bounds()
		assert.Equal(t, 50, bounds.Dx())
		assert.Equal(t, 50, bounds.Dy())
	})

	t.Run("image larger than max", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 400, 400))
		resized := resizeImage(img, 200, 200)

		// Should resize
		bounds := resized.Bounds()
		assert.LessOrEqual(t, bounds.Dx(), 200)
		assert.LessOrEqual(t, bounds.Dy(), 200)
	})

	t.Run("preserves aspect ratio", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 800, 400))
		resized := resizeImage(img, 200, 200)

		bounds := resized.Bounds()
		// Width should be at most 200, height proportionally smaller
		assert.LessOrEqual(t, bounds.Dx(), 200)
		// Aspect ratio should be preserved (2:1)
		ratio := float64(bounds.Dx()) / float64(bounds.Dy())
		assert.InDelta(t, 2.0, ratio, 0.1)
	})
}

func TestExtractPixels(t *testing.T) {
	t.Run("extracts opaque pixels", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 10, 10))
		for y := 0; y < 10; y++ {
			for x := 0; x < 10; x++ {
				img.Set(x, y, color.RGBA{R: 100, G: 150, B: 200, A: 255})
			}
		}

		pixels := extractPixels(img)
		assert.Len(t, pixels, 100) // 10x10

		// All pixels should have the same color
		for _, p := range pixels {
			assert.Equal(t, uint8(100), p.R)
			assert.Equal(t, uint8(150), p.G)
			assert.Equal(t, uint8(200), p.B)
		}
	})

	t.Run("skips transparent pixels", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 10, 10))
		// First row opaque, rest transparent
		for x := 0; x < 10; x++ {
			img.Set(x, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
		for y := 1; y < 10; y++ {
			for x := 0; x < 10; x++ {
				img.Set(x, y, color.RGBA{R: 0, G: 255, B: 0, A: 0})
			}
		}

		pixels := extractPixels(img)
		assert.Len(t, pixels, 10) // Only first row
	})
}

func TestKmeans(t *testing.T) {
	t.Run("single color", func(t *testing.T) {
		pixels := []Color{{R: 100, G: 100, B: 100}}
		for i := 0; i < 99; i++ {
			pixels = append(pixels, Color{R: 100, G: 100, B: 100})
		}

		result := kmeans(pixels, 3, 10)
		require.NotNil(t, result)
		// With all same pixels, we might get fewer clusters
	})

	t.Run("empty pixels", func(t *testing.T) {
		result := kmeans([]Color{}, 3, 10)
		assert.Nil(t, result)
	})

	t.Run("fewer pixels than k", func(t *testing.T) {
		pixels := []Color{
			{R: 255, G: 0, B: 0},
			{R: 0, G: 255, B: 0},
		}
		result := kmeans(pixels, 5, 10)
		// Should reduce k to match pixel count
		assert.Len(t, result, 2)
	})

	t.Run("distinct colors", func(t *testing.T) {
		var pixels []Color

		// Add red pixels
		for i := 0; i < 50; i++ {
			pixels = append(pixels, Color{R: 255, G: 0, B: 0})
		}
		// Add blue pixels
		for i := 0; i < 50; i++ {
			pixels = append(pixels, Color{R: 0, G: 0, B: 255})
		}

		result := kmeans(pixels, 2, 20)
		require.Len(t, result, 2)

		// Total count should equal number of pixels
		totalCount := result[0].Count + result[1].Count
		assert.Equal(t, 100, totalCount)
	})
}

func TestColorDistance(t *testing.T) {
	t.Run("same color", func(t *testing.T) {
		c1 := Color{R: 100, G: 100, B: 100}
		c2 := Color{R: 100, G: 100, B: 100}
		dist := colorDistance(c1, c2)
		assert.Equal(t, 0.0, dist)
	})

	t.Run("opposite colors", func(t *testing.T) {
		black := Color{R: 0, G: 0, B: 0}
		white := Color{R: 255, G: 255, B: 255}
		dist := colorDistance(black, white)
		// sqrt(255^2 + 255^2 + 255^2) = sqrt(195075) ≈ 441.67
		assert.InDelta(t, 441.67, dist, 1.0)
	})

	t.Run("primary colors", func(t *testing.T) {
		red := Color{R: 255, G: 0, B: 0}
		blue := Color{R: 0, G: 0, B: 255}
		dist := colorDistance(red, blue)
		// sqrt(255^2 + 0 + 255^2) = sqrt(130050) ≈ 360.62
		assert.InDelta(t, 360.62, dist, 1.0)
	})
}

func TestColorWithCount(t *testing.T) {
	cwc := ColorWithCount{
		Color: Color{R: 100, G: 150, B: 200},
		Count: 42,
	}

	assert.Equal(t, uint8(100), cwc.Color.R)
	assert.Equal(t, 42, cwc.Count)
}
