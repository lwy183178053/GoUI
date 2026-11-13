package software

import (
	"fmt"
	"image"
	"image/draw"
	"math"

	"github.com/lwy183178053/GoUI/internal/ui"

	xdraw "golang.org/x/image/draw"
)

// SetImageSource 先完整加载目标，成功后再一次性替换，避免失败时留下半成品。
func (r *Renderer) SetImageSource(id, src string) error {
	node, box, err := r.node(id)
	if err != nil {
		return err
	}
	if node.Image == nil {
		return fmt.Errorf("UI id %q is not an image", id)
	}
	loaded, err := r.document.LoadImage(src, node.Image.Playback, node.Source)
	if err != nil {
		return err
	}
	loaded.Fit = node.Image.Fit
	if old := r.imagePlayers[node]; old != nil {
		old.reset() // 取消旧 GIF 已收集但尚未执行的完成回调。
	}
	delete(r.imagePlayers, node)
	delete(r.images, node)
	delete(r.layers, node)
	node.Image = loaded
	if loaded.Animation != nil {
		r.imagePlayers[node] = newImagePlayback(loaded.Animation)
	}
	if box != nil {
		r.invalidate(box.Pixels)
	}
	return nil
}

func (r *Renderer) scaledImage(node *ui.Node, rect image.Rectangle, sourceImage image.Image) *image.RGBA {
	style := node.Image
	if style == nil || sourceImage == nil || rect.Empty() {
		return nil
	}
	if cached := r.images[node]; cached != nil && cached.Bounds().Size() == rect.Size() {
		return cached
	}

	width, height := rect.Dx(), rect.Dy()
	imageRect := image.Rect(0, 0, width, height)
	scaled := image.NewRGBA(imageRect)
	source := sourceImage.Bounds()
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
	xdraw.CatmullRom.Scale(scaled, destination, sourceImage, sourceRect, draw.Over, nil)
	r.images[node] = scaled
	return scaled
}

func paintImage(dst *image.RGBA, rect, limit image.Rectangle, scaled *image.RGBA, radius float64, mask *image.Alpha) {
	if scaled == nil || rect.Empty() {
		return
	}
	clip := rect.Intersect(dst.Bounds()).Intersect(limit)
	if clip.Empty() {
		return
	}
	localClip := clip.Sub(rect.Min)
	if radius <= 0 {
		draw.Draw(dst, clip, scaled, localClip.Min, draw.Over)
		return
	}
	draw.DrawMask(dst, clip, scaled, localClip.Min, mask, localClip.Min, draw.Over)
}
