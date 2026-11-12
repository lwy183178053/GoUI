package goui

import (
	"errors"
	"fmt"

	"github.com/lwy183178053/GoUI/internal/backend/window"
	"github.com/lwy183178053/GoUI/internal/render/software"
	"github.com/lwy183178053/GoUI/internal/ui"
)

// Application 保存一次已加载的 UI 应用。
type Application struct {
	document *ui.Document
	renderer *software.Renderer
	page     *Page
	run      bool
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
	return &Application{document: document, renderer: renderer, page: page}, nil
}

// Page 返回用于查找和操作 XML 组件的页面对象。
func (app *Application) Page() *Page {
	if app == nil {
		return nil
	}
	return app.page
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
	return window.New(window.Options{
		Title:      app.document.AppName,
		Display:    settings.Display,
		Width:      settings.Width,
		Height:     settings.Height,
		Fullscreen: settings.Fullscreen,
	}).Run(app.renderer, nil)
}
