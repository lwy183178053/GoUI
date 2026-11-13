package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"time"
)

// GIFAnimation 保留压缩后的帧，避免加载时把整段动画展开到内存。
type GIFAnimation struct {
	Width      int
	Height     int
	Frames     []GIFFrame
	LoopCount  int
	Background color.Color
}

type GIFFrame struct {
	PNG      []byte
	Bounds   image.Rectangle
	Delay    time.Duration
	Disposal byte
}

// GIFFramePlayer 只合成当前帧，供软件渲染器逐帧调用。
type GIFFramePlayer struct {
	animation        *GIFAnimation
	canvas           *image.RGBA
	previousBounds   image.Rectangle
	previousCanvas   *image.RGBA
	previousDisposal byte
	nextFrame        int
}

func decodeGIF(data []byte) (*GIFAnimation, error) {
	decoded, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if len(decoded.Image) == 0 || decoded.Config.Width <= 0 || decoded.Config.Height <= 0 {
		return nil, fmt.Errorf("GIF has no frames or canvas")
	}

	animation := &GIFAnimation{
		Width: decoded.Config.Width, Height: decoded.Config.Height,
		LoopCount: decoded.LoopCount, Background: gifBackground(decoded),
		Frames: make([]GIFFrame, len(decoded.Image)),
	}
	for i, frame := range decoded.Image {
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, frame); err != nil {
			return nil, fmt.Errorf("compress frame %d: %w", i, err)
		}
		delay := 10 * time.Millisecond
		if i < len(decoded.Delay) && decoded.Delay[i] > 0 {
			delay = time.Duration(decoded.Delay[i]) * 10 * time.Millisecond
		}
		var disposal byte
		if i < len(decoded.Disposal) {
			disposal = decoded.Disposal[i]
		}
		animation.Frames[i] = GIFFrame{
			PNG: encoded.Bytes(), Bounds: frame.Bounds(), Delay: delay, Disposal: disposal,
		}
	}
	return animation, nil
}

// NewPlayer 为一次播放创建画布；隐藏的图片不调用它。
func (a *GIFAnimation) NewPlayer() *GIFFramePlayer {
	player := &GIFFramePlayer{animation: a}
	player.reset()
	return player
}

// Frame 按顺序合成所需帧；跳过的局部帧仍要合成，倒退时从头开始。
func (p *GIFFramePlayer) Frame(index int) (image.Image, error) {
	if p.animation == nil || len(p.animation.Frames) == 0 {
		return nil, fmt.Errorf("GIF player has no frames")
	}
	if index < 0 {
		index = 0
	}
	if index >= len(p.animation.Frames) {
		index = len(p.animation.Frames) - 1
	}
	if index < p.nextFrame {
		p.reset()
	}
	for p.nextFrame <= index {
		if err := p.drawNextFrame(); err != nil {
			return nil, err
		}
	}
	return p.canvas, nil
}

func (p *GIFFramePlayer) reset() {
	bounds := image.Rect(0, 0, p.animation.Width, p.animation.Height)
	p.canvas = image.NewRGBA(bounds)
	draw.Draw(p.canvas, bounds, image.NewUniform(p.animation.Background), image.Point{}, draw.Src)
	p.previousBounds = image.Rectangle{}
	p.previousCanvas = nil
	p.previousDisposal = gif.DisposalNone
	p.nextFrame = 0
}

func (p *GIFFramePlayer) drawNextFrame() error {
	index := p.nextFrame
	if index > 0 {
		switch p.previousDisposal {
		case gif.DisposalBackground:
			draw.Draw(p.canvas, p.previousBounds, image.NewUniform(p.animation.Background), image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			p.canvas = p.previousCanvas
			p.previousCanvas = nil
		}
	}

	frameInfo := p.animation.Frames[index]
	frame, err := png.Decode(bytes.NewReader(frameInfo.PNG))
	if err != nil {
		return fmt.Errorf("decode GIF frame %d: %w", index, err)
	}
	if frameInfo.Disposal == gif.DisposalPrevious {
		p.previousCanvas = cloneRGBA(p.canvas)
	}
	frameBounds := frameInfo.Bounds.Intersect(p.canvas.Bounds())
	draw.Draw(p.canvas, frameBounds, frame, frame.Bounds().Min, draw.Over)
	p.previousBounds = frameBounds
	p.previousDisposal = frameInfo.Disposal
	p.nextFrame++
	return nil
}

func gifBackground(animation *gif.GIF) color.Color {
	// 带透明色的 GIF 使用透明画布，不能用不含 Alpha 的全局调色板填成实色。
	if len(animation.Image) > 0 {
		for _, fill := range animation.Image[0].Palette {
			if _, _, _, alpha := fill.RGBA(); alpha == 0 {
				return color.NRGBA{}
			}
		}
	}
	palette, ok := animation.Config.ColorModel.(color.Palette)
	if !ok || int(animation.BackgroundIndex) >= len(palette) {
		return color.NRGBA{}
	}
	return color.NRGBAModel.Convert(palette[animation.BackgroundIndex])
}

func cloneRGBA(source *image.RGBA) *image.RGBA {
	clone := image.NewRGBA(source.Bounds())
	draw.Draw(clone, clone.Bounds(), source, source.Bounds().Min, draw.Src)
	return clone
}
