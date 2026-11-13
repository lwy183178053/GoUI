// Package testui 提供小尺寸测试资源，不依赖窗口或整套 GIF 解码。
package testui

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	goui "github.com/lwy183178053/GoUI"
	"golang.org/x/image/font/gofont/goregular"
)

func Open(t testing.TB) (*goui.Application, string, func(context.Context, func(*goui.Page) error) error) {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	uiDirectory := filepath.Join(filepath.Dir(filename), "..", "..", "ui")
	directory := t.TempDir()
	for _, name := range []string{"gif", "fonts"} {
		if err := os.Mkdir(filepath.Join(directory, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mainXML, err := os.ReadFile(filepath.Join(uiDirectory, "main.xml"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"main.xml":          mainXML,
		"config.xml":        []byte(`<config><fonts><font name="default" src="fonts/default.ttf"/></fonts></config>`),
		"fonts/default.ttf": goregular.TTF,
	}
	palette := color.Palette{color.Black, color.White}
	first := image.NewPaletted(image.Rect(0, 0, 2, 1), palette)
	second := image.NewPaletted(image.Rect(0, 0, 2, 1), palette)
	first.SetColorIndex(0, 0, 1)
	second.SetColorIndex(1, 0, 1)
	var data bytes.Buffer
	// 每张只占两个像素；测试播放完成由回调触发，不依赖大尺寸素材。
	if err := gif.EncodeAll(&data, &gif.GIF{Image: []*image.Paletted{first, second, first}, Delay: []int{1000, 1000, 1000}, LoopCount: 0}); err != nil {
		t.Fatal(err)
	}
	gifFiles, err := filepath.Glob(filepath.Join(uiDirectory, "gif", "*.gif"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range gifFiles {
		files[filepath.Join("gif", filepath.Base(file))] = data.Bytes()
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(directory, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	app, err := goui.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	var mainLoop sync.Mutex
	dispatch := func(ctx context.Context, update func(*goui.Page) error) error {
		mainLoop.Lock()
		defer mainLoop.Unlock()
		if err := ctx.Err(); err != nil {
			return err
		}
		return update(app.Page())
	}
	return app, directory, dispatch
}
