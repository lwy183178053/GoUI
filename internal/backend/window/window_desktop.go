//go:build windows || linux

package window

import (
	"errors"
	"image"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lwy183178053/GoUI/internal/backend"
)

var errWindowClosed = errors.New("window closed")

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

func (p *presenter) Run(source backend.FrameSource) error {
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

	// 设备窗口失去焦点后仍需接收网络控制并继续播放动画。
	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetWindowTitle(p.options.Title)
	ebiten.SetMonitor(monitor)
	ebiten.SetWindowSize(width, height)
	ebiten.SetWindowDecorated(p.options.Decorated)
	resizingMode := ebiten.WindowResizingModeDisabled
	if p.options.Resizable {
		resizingMode = ebiten.WindowResizingModeEnabled
	}
	ebiten.SetWindowResizingMode(resizingMode)
	minWidth, minHeight := -1, -1
	if !p.options.Fullscreen {
		minWidth, minHeight = p.options.MinWidth, p.options.MinHeight
	}
	if minWidth == 0 {
		minWidth = -1
	}
	if minHeight == 0 {
		minHeight = -1
	}
	ebiten.SetWindowSizeLimits(minWidth, minHeight, -1, -1)
	ebiten.SetFullscreen(p.options.Fullscreen)
	ebiten.SetWindowClosingHandled(true)
	defer ebiten.SetWindowClosingHandled(false)

	game := &preview{source: source, closeRequested: &p.closeRequested}
	defer game.close()
	err = ebiten.RunGame(game)
	if errors.Is(err, errWindowClosed) {
		return nil
	}
	return err
}

func (p *presenter) Minimize() error {
	ebiten.MinimizeWindow()
	return nil
}

func (p *presenter) ToggleMaximize() error {
	if !p.options.Resizable {
		return errors.New("window cannot be maximized when resizable is false")
	}
	if ebiten.IsWindowMaximized() || ebiten.IsWindowMinimized() {
		ebiten.RestoreWindow()
	} else {
		ebiten.MaximizeWindow()
	}
	return nil
}

func (p *presenter) Close() error {
	p.closeRequested.Store(true)
	return nil
}

type preview struct {
	source         backend.FrameSource
	frame          *image.RGBA
	texture        *ebiten.Image
	err            error
	mouseDown      bool
	closeRequested *atomic.Bool
}

func (g *preview) Update() error {
	if g.err != nil {
		return g.err
	}
	if ebiten.IsWindowBeingClosed() || (g.closeRequested != nil && g.closeRequested.Load()) {
		return errWindowClosed
	}
	if updater, ok := g.source.(backend.Updater); ok {
		if err := updater.Update(); err != nil {
			g.err = err
			return err
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
	scale := 1.0
	if monitor := ebiten.Monitor(); monitor != nil {
		scale = monitor.DeviceScaleFactor()
	}
	return renderSize(width, height, scale)
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
