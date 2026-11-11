// Package window 在原生 Windows 窗口中显示预览画面。
package window

import "goui/internal/backend"

type Options struct {
	Title string
}

type presenter struct{ options Options }

func New(options Options) backend.Presenter {
	if options.Title == "" {
		options.Title = "GoUI Preview"
	}
	return &presenter{options: options}
}
