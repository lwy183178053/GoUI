// Package window 使用 Ebitengine 管理桌面窗口和显示器。
package window

import (
	"fmt"
	"math"
	"sync/atomic"

	"github.com/lwy183178053/GoUI/internal/backend"
)

type Options struct {
	Title      string
	Display    int
	Width      int
	Height     int
	Fullscreen bool
	Decorated  bool
	Resizable  bool
	MinWidth   int
	MinHeight  int
}

type presenter struct {
	options        Options
	closeRequested atomic.Bool
}

// Presenter 管理桌面窗口，并提供自定义窗口按钮需要的操作。
type Presenter interface {
	backend.Presenter
	Minimize() error
	ToggleMaximize() error
	Close() error
}

type Display struct {
	Index  int
	Name   string
	Width  int
	Height int
}

func New(options Options) Presenter {
	if options.Title == "" {
		options.Title = "GoUI"
	}
	return &presenter{options: options}
}

func validateDisplayIndex(index, count int) error {
	if index < 0 || index >= count {
		return fmt.Errorf("display index %d is unavailable; detected %d display(s)", index, count)
	}
	return nil
}

func resolveWindowSize(width, height, displayWidth, displayHeight int, fullscreen bool) (int, int, error) {
	if fullscreen {
		width, height = displayWidth, displayHeight
	}
	if width <= 0 || height <= 0 {
		return 0, 0, fmt.Errorf("invalid window size %dx%d", width, height)
	}
	return width, height, nil
}

// renderSize 把窗口逻辑尺寸换算成显示器上的渲染像素。
func renderSize(width, height int, scale float64) (int, int) {
	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		scale = 1
	}
	return max(1, int(math.Round(float64(width)*scale))),
		max(1, int(math.Round(float64(height)*scale)))
}
