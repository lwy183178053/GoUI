package goui

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/lwy183178053/GoUI/internal/backend/window"
	"github.com/lwy183178053/GoUI/internal/render/software"
	"github.com/lwy183178053/GoUI/internal/ui"
)

// Application 保存一次已加载的 UI 应用。
type Application struct {
	document *ui.Document
	renderer *software.Renderer
	page     *Page
	window   window.Presenter
	run      bool
	running  atomic.Bool
	updates  chan pageUpdate
	done     chan struct{}
}

// Open 从 UI 目录加载 config.xml、main.xml 和相关资源。
func Open(directory string) (*Application, error) {
	document, err := ui.Load(directory)
	if err != nil {
		return nil, err
	}
	renderer, err := software.New(document)
	if err != nil {
		return nil, err
	}
	page := &Page{renderer: renderer}
	return &Application{
		document: document, renderer: renderer, page: page,
		updates: make(chan pageUpdate, 64), done: make(chan struct{}),
	}, nil
}

// Page 返回用于查找和操作 XML 组件的页面对象。
func (app *Application) Page() *Page {
	if app == nil {
		return nil
	}
	return app.page
}

func (app *Application) windowControls() (window.Presenter, error) {
	if app == nil || app.window == nil {
		return nil, errors.New("window is not running")
	}
	return app.window, nil
}

// MinimizeWindow 最小化当前桌面窗口。
func (app *Application) MinimizeWindow() error {
	controls, err := app.windowControls()
	if err != nil {
		return err
	}
	return controls.Minimize()
}

// ToggleMaximizeWindow 在最大化与还原之间切换；窗口需要允许缩放。
func (app *Application) ToggleMaximizeWindow() error {
	controls, err := app.windowControls()
	if err != nil {
		return err
	}
	return controls.ToggleMaximize()
}

// CloseWindow 请求正常关闭当前桌面窗口。
func (app *Application) CloseWindow() error {
	controls, err := app.windowControls()
	if err != nil {
		return err
	}
	return controls.Close()
}

// Run 按 config.xml 中的窗口设置启动应用。
func (app *Application) Run() error {
	if app == nil || app.renderer == nil || app.document == nil {
		return errors.New("application is not open")
	}
	if app.run {
		return errors.New("application can only be run once")
	}
	app.run = true
	defer app.renderer.Close()

	settings := app.document.Window
	if settings.Backend == "drm" {
		return errors.New("DRM backend is not implemented yet")
	}
	if settings.Backend != "desktop" {
		return fmt.Errorf("unsupported backend %q", settings.Backend)
	}
	presenter := window.New(window.Options{
		Title:      app.document.AppName,
		Display:    settings.Display,
		Width:      settings.Width,
		Height:     settings.Height,
		Fullscreen: settings.Fullscreen,
		Decorated:  settings.Decorated,
		Resizable:  settings.Resizable,
		MinWidth:   settings.MinWidth,
		MinHeight:  settings.MinHeight,
	})
	app.window = presenter
	defer func() { app.window = nil }()
	app.running.Store(true)
	defer func() {
		app.running.Store(false)
		close(app.done)
	}()
	source := &applicationSource{Renderer: app.renderer, page: app.page, updates: app.updates}
	return presenter.Run(source)
}
