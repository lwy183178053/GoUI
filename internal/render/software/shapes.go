package software

import (
	"image"
	"math"
)

type roundedMaskKey struct {
	size   image.Point
	radius float64
}

func (r *Renderer) roundedMask(size image.Point, radius float64) *image.Alpha {
	key := roundedMaskKey{size: size, radius: radius}
	if mask := r.maskCache[key]; mask != nil {
		return mask
	}
	mask := roundedMaskForSize(size, radius)
	r.maskCache[key] = mask
	return mask
}

func roundedMaskForSize(size image.Point, radius float64) *image.Alpha {
	if size.X <= 0 || size.Y <= 0 {
		return image.NewAlpha(image.Rectangle{})
	}
	radius = math.Min(radius, float64(min(size.X, size.Y))/2)
	mask := image.NewAlpha(image.Rect(0, 0, size.X, size.Y))
	for i := range mask.Pix {
		mask.Pix[i] = 255
	}
	if radius <= 0 {
		return mask
	}

	left, right := radius, float64(size.X)-radius
	top, bottom := radius, float64(size.Y)-radius
	leftEnd := min(size.X, int(math.Ceil(left)))
	rightStart := max(0, int(math.Floor(right)))
	topEnd := min(size.Y, int(math.Ceil(top)))
	bottomStart := max(0, int(math.Floor(bottom)))
	// 用像素中心到圆角边界的距离估算覆盖率，只计算四角区域。
	setCorner := func(x, y int) {
		px, py := float64(x)+0.5, float64(y)+0.5
		dx := math.Max(math.Max(left-px, px-right), 0)
		dy := math.Max(math.Max(top-py, py-bottom), 0)
		coverage := 0.5 - (math.Hypot(dx, dy) - radius)
		coverage = math.Max(0, math.Min(1, coverage))
		mask.Pix[y*mask.Stride+x] = uint8(math.Round(coverage * 255))
	}
	for y := 0; y < topEnd; y++ {
		for x := 0; x < leftEnd; x++ {
			setCorner(x, y)
		}
		for x := rightStart; x < size.X; x++ {
			setCorner(x, y)
		}
	}
	for y := bottomStart; y < size.Y; y++ {
		for x := 0; x < leftEnd; x++ {
			setCorner(x, y)
		}
		for x := rightStart; x < size.X; x++ {
			setCorner(x, y)
		}
	}
	return mask
}
