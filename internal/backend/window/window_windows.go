//go:build windows

package window

import (
	"fmt"
	"image"

	"goui/internal/backend"

	"github.com/hajimehoshi/ebiten/v2"
)

func (p *presenter) Run(source backend.FrameSource) error {
	monitors := ebiten.AppendMonitors(nil)
	if len(monitors) < 2 {
		return fmt.Errorf("需要连接至少两块屏幕，当前检测到 %d 块", len(monitors))
	}

	monitor := monitors[1]
	width, height := monitor.Size()
	if width <= 0 || height <= 0 {
		return fmt.Errorf("无法读取第 2 块屏幕的尺寸：%dx%d", width, height)
	}

	ebiten.SetWindowTitle(p.options.Title)
	ebiten.SetMonitor(monitor)
	ebiten.SetWindowSize(width, height)
	ebiten.SetFullscreen(true)
	fmt.Printf("GoUI Preview 已在第 2 块屏幕全屏启动，渲染尺寸 %dx%d（按 Alt+F4 退出）\n", width, height)

	game := &preview{source: source}
	defer game.close()
	return ebiten.RunGame(game)
}

type preview struct {
	source    backend.FrameSource
	frame     *image.RGBA
	texture   *ebiten.Image
	err       error
	mouseDown bool
}

func (g *preview) Update() error {
	if g.err != nil {
		return g.err
	}
	handler, ok := g.source.(backend.PointerHandler)
	if !ok {
		return nil
	}
	pressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	if pressed && !g.mouseDown {
		x, y := ebiten.CursorPosition()
		handler.PointerDown(x, y)
		g.mouseDown = true
	} else if !pressed && g.mouseDown {
		x, y := ebiten.CursorPosition()
		g.mouseDown = false
		g.err = handler.PointerUp(x, y)
	}
	return g.err
}

func (g *preview) Layout(width, height int) (int, int) {
	return max(1, width), max(1, height)
}

func (g *preview) Draw(screen *ebiten.Image) {
	if g.err != nil {
		return
	}
	frame, err := g.source.Frame(screen.Bounds().Dx(), screen.Bounds().Dy())
	if err != nil {
		g.err = err
		return
	}
	if frame != g.frame {
		if g.texture != nil && g.texture.Bounds().Size() != frame.Bounds().Size() {
			g.texture.Deallocate()
			g.texture = nil
		}
		if g.texture == nil {
			g.texture = ebiten.NewImage(frame.Rect.Dx(), frame.Rect.Dy())
		}
		g.texture.WritePixels(frame.Pix)
		g.frame = frame
	}
	screen.DrawImage(g.texture, nil)
}

func (g *preview) close() {
	if g.texture != nil {
		g.texture.Deallocate()
	}
}
