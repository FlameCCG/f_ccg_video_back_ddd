package ai

import (
	"fmt"
	"math"
	"strings"
)

// minSeedreamImagePixels Seedream 4.5/5 系列要求的最小出图像素数。
const minSeedreamImagePixels = 3686400

// ResolveImageSize 推导下发给上游的 size 参数。
//
// 优先使用显式 size；否则由 resolution + aspectRatio 推导，
// 并对 Seedream 4.5/5 系列补足最小像素约束。
func ResolveImageSize(model, size, resolution, ratio string) string {
	size = strings.TrimSpace(size)
	if size != "" {
		return normalizeSeedreamImageSize(model, size)
	}

	resolution = strings.ToLower(strings.TrimSpace(resolution))
	if resolution == "" {
		resolution = "2k"
	}

	ratio = NormalizeVideoRatio(ratio)
	if ratio == "" || ratio == "adaptive" {
		switch resolution {
		case "1k":
			if requiresSeedreamMinImagePixels(model) {
				return "2K"
			}

			return "1K"
		case "3k":
			return "3K"
		case "4k":
			return "4K"
		default:
			return "2K"
		}
	}

	maxSide := 2048

	switch resolution {
	case "1k":
		maxSide = 1024
	case "3k":
		maxSide = 3072
	case "4k":
		maxSide = 4096
	}

	if requiresSeedreamMinImagePixels(model) {
		minMaxSide := getSeedreamMinMaxSide(ratio)
		if maxSide < minMaxSide {
			maxSide = minMaxSide
		}
	}

	width, height := dimensionsForRatio(maxSide, ratio)

	return fmt.Sprintf("%dx%d", width, height)
}

func normalizeSeedreamImageSize(model, size string) string {
	if !requiresSeedreamMinImagePixels(model) {
		return size
	}

	normalized := strings.ToLower(strings.TrimSpace(size))
	switch normalized {
	case "1k":
		return "2K"
	case "2k", "3k", "4k":
		return strings.ToUpper(normalized)
	}

	width, height, ok := parseImageSize(size)
	if !ok || width*height >= minSeedreamImagePixels {
		return size
	}

	if ratio := ratioForDimensions(width, height); ratio != "" {
		maxSide := getSeedreamMinMaxSide(ratio)
		width, height = dimensionsForRatio(maxSide, ratio)

		return fmt.Sprintf("%dx%d", width, height)
	}

	scale := math.Sqrt(float64(minSeedreamImagePixels) / float64(width*height))
	width = ceilToMultiple(int(math.Ceil(float64(width)*scale)), 8)

	height = ceilToMultiple(int(math.Ceil(float64(height)*scale)), 8)
	for width*height < minSeedreamImagePixels {
		if width >= height {
			height += 8
		} else {
			width += 8
		}
	}

	return fmt.Sprintf("%dx%d", width, height)
}

func requiresSeedreamMinImagePixels(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))

	return strings.Contains(model, "seedream-4-5") || strings.Contains(model, "seedream-5")
}

func getSeedreamMinMaxSide(ratio string) int {
	switch ratio {
	case "1:1":
		return 2048
	case "16:9", "9:16":
		return 2560
	case "4:3", "3:4":
		return 2304
	case "21:9", "9:21":
		return 3024
	default:
		parts := strings.Split(ratio, ":")
		if len(parts) == 2 {
			var w, h float64

			_, _ = fmt.Sscanf(parts[0], "%f", &w)

			_, _ = fmt.Sscanf(parts[1], "%f", &h)

			if w > 0 && h > 0 {
				r := w / h
				if h > w {
					r = h / w
				}

				minMaxSide := int(math.Ceil(math.Sqrt(float64(minSeedreamImagePixels) * r)))

				return (minMaxSide + 7) / 8 * 8
			}
		}

		return 2048
	}
}

func parseImageSize(size string) (int, int, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(size)), "x")
	if len(parts) != 2 {
		return 0, 0, false
	}

	var width, height int
	if _, err := fmt.Sscanf(strings.TrimSpace(parts[0]), "%d", &width); err != nil {
		return 0, 0, false
	}

	if _, err := fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &height); err != nil {
		return 0, 0, false
	}

	return width, height, width > 0 && height > 0
}

func ratioForDimensions(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	current := float64(width) / float64(height)

	for _, ratio := range []string{"1:1", "16:9", "9:16", "4:3", "3:4", "21:9"} {
		parts := strings.Split(ratio, ":")

		var w, h float64

		_, _ = fmt.Sscanf(parts[0], "%f", &w)

		_, _ = fmt.Sscanf(parts[1], "%f", &h)

		if math.Abs(current-w/h) < 0.01 {
			return ratio
		}
	}

	return ""
}

func ceilToMultiple(value, multiple int) int {
	if multiple <= 0 {
		return value
	}

	return ((value + multiple - 1) / multiple) * multiple
}

func dimensionsForRatio(maxSide int, ratio string) (int, int) {
	parts := strings.Split(ratio, ":")
	if len(parts) != 2 {
		return maxSide, maxSide
	}

	var w, h float64

	_, _ = fmt.Sscanf(parts[0], "%f", &w)

	_, _ = fmt.Sscanf(parts[1], "%f", &h)

	if w <= 0 || h <= 0 {
		return maxSide, maxSide
	}

	if w >= h {
		return maxSide, int(math.Round(float64(maxSide)*h/w/8) * 8)
	}

	return int(math.Round(float64(maxSide)*w/h/8) * 8), maxSide
}
