package ui

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
)

// LoadImage 按需读取一张图片，XML 加载和 Image.SetSource 共用此入口。
// 成功前不修改原节点，也不缓存所有候选图片。
func (d *Document) LoadImage(src, playback string, source Source) (*ImageStyle, error) {
	if d.files == nil {
		return nil, at(source, "document has no resource file system")
	}
	style := &ImageStyle{Source: src, Playback: playback, Fit: "stretch"}
	asset, err := assetPath(src)
	if err != nil {
		return nil, at(source, "image src must be relative to the UI directory")
	}
	data, err := fs.ReadFile(d.files, asset)
	if err != nil {
		return nil, at(source, "read image %q: %w", src, err)
	}
	_, format, decodeErr := image.DecodeConfig(bytes.NewReader(data))
	if decodeErr != nil {
		return nil, at(source, "decode image %q: %w", src, decodeErr)
	}
	if format == "gif" {
		animation, err := decodeGIF(data)
		if err != nil {
			return nil, at(source, "decode GIF %q: %w", src, err)
		}
		if playback == "segmented" {
			animation.Segments, err = parseGIFSegments(src, animation)
			if err != nil {
				return nil, at(source, "GIF %q: %w", src, err)
			}
		}
		if len(animation.Frames) > 1 || animation.Segments != nil {
			style.Animation = animation
		} else {
			style.Image, err = animation.NewPlayer().Frame(0)
			if err != nil {
				return nil, at(source, "decode GIF %q: %w", src, err)
			}
		}
	} else {
		if playback == "segmented" {
			return nil, at(source, "playback=segmented requires a GIF image, got %q", src)
		}
		decoded, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, at(source, "decode image %q: %w", src, err)
		}
		if decoded.Bounds().Empty() {
			return nil, at(source, "image %q has no pixels", src)
		}
		style.Image = decoded
	}
	d.addResource(asset)
	return style, nil
}
