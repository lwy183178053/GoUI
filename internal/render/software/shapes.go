package software

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

func fillRoundedRect(dst *image.RGBA, rect image.Rectangle, fill color.NRGBA, radius float64) {
	fillRoundedRectClip(dst, rect, fill, radius, dst.Bounds())
}

func fillRoundedRectClip(dst *image.RGBA, rect image.Rectangle, fill color.NRGBA, radius float64, limit image.Rectangle) {
	clip := rect.Intersect(dst.Bounds()).Intersect(limit)
	if clip.Empty() || fill.A == 0 {
		return
	}
	if radius <= 0 {
		draw.Draw(dst, clip, image.NewUniform(fill), image.Point{}, draw.Over)
		return
	}
	mask := roundedMask(rect, clip, radius)
	draw.DrawMask(dst, clip, image.NewUniform(fill), image.Point{}, mask, clip.Min, draw.Over)
}

func roundedMask(rect, clip image.Rectangle, radius float64) *image.Alpha {
	radius = math.Min(radius, float64(min(rect.Dx(), rect.Dy()))/2)
	mask := image.NewAlpha(clip)
	left, right := float64(rect.Min.X)+radius, float64(rect.Max.X)-radius
	top, bottom := float64(rect.Min.Y)+radius, float64(rect.Max.Y)-radius
	// 只对圆角区域做超采样，矩形内部直接填充。
	const samples = 4
	for y := clip.Min.Y; y < clip.Max.Y; y++ {
		for x := clip.Min.X; x < clip.Max.X; x++ {
			coverage := samples * samples
			if !((float64(x) >= left && float64(x+1) <= right) || (float64(y) >= top && float64(y+1) <= bottom)) {
				coverage = 0
				for sy := range samples {
					py := float64(y) + (float64(sy)+0.5)/samples
					dy := math.Max(math.Max(top-py, py-bottom), 0)
					for sx := range samples {
						px := float64(x) + (float64(sx)+0.5)/samples
						dx := math.Max(math.Max(left-px, px-right), 0)
						if dx*dx+dy*dy <= radius*radius {
							coverage++
						}
					}
				}
			}
			mask.SetAlpha(x, y, color.Alpha{A: uint8((coverage*255 + samples*samples/2) / (samples * samples))})
		}
	}
	return mask
}
