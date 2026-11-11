package software

import (
	"image"
	"image/draw"
	"math"

	"goui/internal/ui"

	xdraw "golang.org/x/image/draw"
)

func paintImage(dst *image.RGBA, rect, limit image.Rectangle, style *ui.ImageStyle, radius float64) {
	if style == nil || style.Image == nil || rect.Empty() {
		return
	}
	clip := rect.Intersect(dst.Bounds()).Intersect(limit)
	if clip.Empty() {
		return
	}

	width, height := rect.Dx(), rect.Dy()
	imageRect := image.Rect(0, 0, width, height)
	scaled := image.NewRGBA(imageRect)
	source := style.Image.Bounds()
	sourceRect := source
	destination := imageRect

	switch style.Fit {
	case "contain":
		scale := math.Min(float64(width)/float64(source.Dx()), float64(height)/float64(source.Dy()))
		destWidth := max(1, int(math.Round(float64(source.Dx())*scale)))
		destHeight := max(1, int(math.Round(float64(source.Dy())*scale)))
		destination = image.Rect((width-destWidth)/2, (height-destHeight)/2, (width+destWidth)/2, (height+destHeight)/2)
	case "cover":
		sourceWidth, sourceHeight := source.Dx(), source.Dy()
		if float64(sourceWidth)/float64(sourceHeight) > float64(width)/float64(height) {
			cropWidth := max(1, int(math.Round(float64(sourceHeight)*float64(width)/float64(height))))
			left := source.Min.X + (sourceWidth-cropWidth)/2
			sourceRect = image.Rect(left, source.Min.Y, left+cropWidth, source.Max.Y)
		} else {
			cropHeight := max(1, int(math.Round(float64(sourceWidth)*float64(height)/float64(width))))
			top := source.Min.Y + (sourceHeight-cropHeight)/2
			sourceRect = image.Rect(source.Min.X, top, source.Max.X, top+cropHeight)
		}
	}
	xdraw.CatmullRom.Scale(scaled, destination, style.Image, sourceRect, draw.Over, nil)

	localClip := clip.Sub(rect.Min)
	if radius <= 0 {
		draw.Draw(dst, clip, scaled, localClip.Min, draw.Over)
		return
	}
	mask := roundedMask(imageRect, localClip, radius)
	draw.DrawMask(dst, clip, scaled, localClip.Min, mask, localClip.Min, draw.Over)
}
