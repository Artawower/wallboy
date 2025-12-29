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

type Color struct {
	R, G, B uint8
}

func (c Color) Hex() string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

type ColorWithCount struct {
	Color Color
	Count int
}

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

	resized := resizeImage(img, 200, 200)
	pixels := extractPixels(resized)
	if len(pixels) == 0 {
		return nil, fmt.Errorf("no pixels extracted from image")
	}

	clusters := kmeans(pixels, topN, 20)

	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Count > clusters[j].Count
	})

	colors := make([]Color, len(clusters))
	for i, c := range clusters {
		colors[i] = c.Color
	}

	return colors, nil
}

func resizeImage(img image.Image, maxWidth, maxHeight int) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	scaleX := float64(maxWidth) / float64(width)
	scaleY := float64(maxHeight) / float64(height)
	scale := math.Min(scaleX, scaleY)

	if scale >= 1 {
		return img
	}

	newWidth := int(float64(width) * scale)
	newHeight := int(float64(height) * scale)

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

func extractPixels(img image.Image) []Color {
	bounds := img.Bounds()
	var pixels []Color

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if a < 32768 {
				continue
			}
			pixels = append(pixels, Color{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
			})
		}
	}

	return pixels
}

func kmeans(pixels []Color, k int, maxIterations int) []ColorWithCount {
	if len(pixels) < k {
		k = len(pixels)
	}

	if k == 0 {
		return nil
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	centroids := make([]Color, k)
	perm := rng.Perm(len(pixels))
	for i := 0; i < k; i++ {
		centroids[i] = pixels[perm[i]]
	}

	var assignments []int
	for iter := 0; iter < maxIterations; iter++ {
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
				newCentroids[i] = centroids[i]
			}
		}

		centroids = newCentroids

		if !changed {
			break
		}
	}

	result := make([]ColorWithCount, k)
	for i := 0; i < k; i++ {
		result[i].Color = centroids[i]
	}
	for _, cluster := range assignments {
		result[cluster].Count++
	}

	return result
}

func colorDistance(c1, c2 Color) float64 {
	dr := float64(c1.R) - float64(c2.R)
	dg := float64(c1.G) - float64(c2.G)
	db := float64(c1.B) - float64(c2.B)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}
