// Package colors provides color analysis for images using k-means clustering.
package colors

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"math/rand"
	"os"
	"sort"
	"time"

	_ "golang.org/x/image/webp"
)

// Color represents an RGB color.
type Color struct {
	R, G, B uint8
}

// Hex returns the hex representation of the color.
func (c Color) Hex() string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// ColorWithCount represents a color with its occurrence count.
type ColorWithCount struct {
	Color Color
	Count int
}

// Analyze extracts dominant colors from an image using k-means clustering.
func Analyze(path string, topN int) ([]Color, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Resize image for faster processing
	resized := resizeImage(img, 200, 200)

	// Extract all pixels
	pixels := extractPixels(resized)
	if len(pixels) == 0 {
		return nil, fmt.Errorf("no pixels extracted from image")
	}

	// Run k-means clustering
	clusters := kmeans(pixels, topN, 20)

	// Sort by count (most common first)
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Count > clusters[j].Count
	})

	// Extract just the colors
	colors := make([]Color, len(clusters))
	for i, c := range clusters {
		colors[i] = c.Color
	}

	return colors, nil
}

// resizeImage resizes an image to fit within maxWidth x maxHeight.
func resizeImage(img image.Image, maxWidth, maxHeight int) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate scaling factor
	scaleX := float64(maxWidth) / float64(width)
	scaleY := float64(maxHeight) / float64(height)
	scale := math.Min(scaleX, scaleY)

	if scale >= 1 {
		return img
	}

	newWidth := int(float64(width) * scale)
	newHeight := int(float64(height) * scale)

	// Simple nearest-neighbor resize
	resized := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			srcX := int(float64(x) / scale)
			srcY := int(float64(y) / scale)
			resized.Set(x, y, img.At(bounds.Min.X+srcX, bounds.Min.Y+srcY))
		}
	}

	return resized
}

// extractPixels extracts all pixels from an image as Color values.
func extractPixels(img image.Image) []Color {
	bounds := img.Bounds()
	var pixels []Color

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			// Skip transparent pixels
			if a < 32768 {
				continue
			}
			// Convert from 16-bit to 8-bit
			pixels = append(pixels, Color{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
			})
		}
	}

	return pixels
}

// kmeans performs k-means clustering on a set of colors.
func kmeans(pixels []Color, k int, maxIterations int) []ColorWithCount {
	if len(pixels) < k {
		k = len(pixels)
	}

	if k == 0 {
		return nil
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Initialize centroids randomly
	centroids := make([]Color, k)
	perm := rng.Perm(len(pixels))
	for i := 0; i < k; i++ {
		centroids[i] = pixels[perm[i]]
	}

	// Iterate
	var assignments []int
	for iter := 0; iter < maxIterations; iter++ {
		// Assign each pixel to nearest centroid
		assignments = make([]int, len(pixels))
		for i, pixel := range pixels {
			minDist := math.MaxFloat64
			minIdx := 0
			for j, centroid := range centroids {
				dist := colorDistance(pixel, centroid)
				if dist < minDist {
					minDist = dist
					minIdx = j
				}
			}
			assignments[i] = minIdx
		}

		// Update centroids
		newCentroids := make([]Color, k)
		counts := make([]int, k)
		sums := make([][3]int64, k)

		for i, pixel := range pixels {
			cluster := assignments[i]
			counts[cluster]++
			sums[cluster][0] += int64(pixel.R)
			sums[cluster][1] += int64(pixel.G)
			sums[cluster][2] += int64(pixel.B)
		}

		changed := false
		for i := 0; i < k; i++ {
			if counts[i] > 0 {
				newCentroids[i] = Color{
					R: uint8(sums[i][0] / int64(counts[i])),
					G: uint8(sums[i][1] / int64(counts[i])),
					B: uint8(sums[i][2] / int64(counts[i])),
				}
				if newCentroids[i] != centroids[i] {
					changed = true
				}
			} else {
				// Keep old centroid if no pixels assigned
				newCentroids[i] = centroids[i]
			}
		}

		centroids = newCentroids

		if !changed {
			break
		}
	}

	// Count pixels in each cluster
	result := make([]ColorWithCount, k)
	for i := 0; i < k; i++ {
		result[i].Color = centroids[i]
	}
	for _, cluster := range assignments {
		result[cluster].Count++
	}

	return result
}

// colorDistance calculates the Euclidean distance between two colors.
func colorDistance(c1, c2 Color) float64 {
	dr := float64(c1.R) - float64(c2.R)
	dg := float64(c1.G) - float64(c2.G)
	db := float64(c1.B) - float64(c2.B)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}
