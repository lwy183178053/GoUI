//go:build windows || linux

package window

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lwy183178053/GoUI/internal/backend"
)

func Displays() ([]Display, error) {
	monitors := ebiten.AppendMonitors(nil)
	result := make([]Display, 0, len(monitors))
	for index, monitor := range monitors {
		width, height := monitor.Size()
		result = append(result, Display{
			Index: index, Name: monitor.Name(), Width: width, Height: height,
		})
	}
	return result, nil
}

func (p *presenter) Run(source backend.FrameSource, onReady func() error) error {
	monitors := ebiten.AppendMonitors(nil)
	if err := validateDisplayIndex(p.options.Display, len(monitors)); err != nil {
		return err
	}
	monitor := monitors[p.options.Display]
	displayWidth, displayHeight := monitor.Size()
	width, height, err := resolveWindowSize(p.options.Width, p.options.Height, displayWidth, displayHeight, p.options.Fullscreen)
	if err != nil {
		return err
	}

	ebiten.SetWindowTitle(p.options.Title)
	ebiten.SetMonitor(monitor)
	ebiten.SetWindowSize(width, height)
	ebiten.SetFullscreen(p.options.Fullscreen)

	game := &preview{source: source, onReady: onReady}
	defer game.close()
	return ebiten.RunGame(game)
}

type preview struct {
	source    backend.FrameSource
	onReady   func() error
	frame     *image.RGBA
	texture   *ebiten.Image
	err       error
	mouseDown bool
	ready     bool
}

func (g *preview) Update() error {
	if g.err != nil {
		return g.err
	}
	if !g.ready {
		g.ready = true
		if g.onReady != nil {
			if err := g.onReady(); err != nil {
				g.err = err
				return err
			}
		}
	}
	handler, ok := g.source.(backend.PointerHandler)
	if !ok {
		return nil
	}
	x, y := ebiten.CursorPosition()
	if err := handler.PointerMove(x, y); err != nil {
		g.err = err
		return err
	}
	pressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	if pressed && !g.mouseDown {
		if err := handler.PointerDown(x, y); err != nil {
			g.err = err
			return err
		}
		g.mouseDown = true
	} else if !pressed && g.mouseDown {
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
