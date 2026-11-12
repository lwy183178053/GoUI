// Package window 使用 Ebitengine 管理桌面窗口和显示器。
package window

import (
	"fmt"

	"github.com/lwy183178053/GoUI/internal/backend"
)

type Options struct {
	Title      string
	Display    int
	Width      int
	Height     int
	Fullscreen bool
}

type presenter struct{ options Options }

type Display struct {
	Index  int
	Name   string
	Width  int
	Height int
}

func New(options Options) backend.Presenter {
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
