package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"sync"
	"testing"

	goui "github.com/lwy183178053/GoUI"
	"golang.org/x/image/font/gofont/goregular"
)

// 使用两像素测试 GIF，接口测试无需反复解码整套大素材。
func testRobot(t *testing.T) (*robot, string) {
	t.Helper()
	directory := t.TempDir()
	for _, subdirectory := range []string{"gif", "fonts"} {
		if err := os.Mkdir(filepath.Join(directory, subdirectory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mainXML, err := os.ReadFile("ui/main.xml")
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
	if err := gif.EncodeAll(&data, &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{4, 4}, LoopCount: 0}); err != nil {
		t.Fatal(err)
	}
	for _, item := range expressions {
		files[filepath.Join("gif", item.File)] = data.Bytes()
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
	// 测试中串行执行回调，模拟窗口主循环；真实运行使用 app.Dispatch。
	dispatch := func(ctx context.Context, update func(*goui.Page) error) error {
		mainLoop.Lock()
		defer mainLoop.Unlock()
		if err := ctx.Err(); err != nil {
			return err
		}
		return update(app.Page())
	}
	robot, err := newRobot(app.Page(), dispatch)
	if err != nil {
		t.Fatal(err)
	}
	return robot, directory
}

func TestRobotStartsWithNormalExpression(t *testing.T) {
	robot, _ := testRobot(t)
	got := robot.state()
	if got.Vector != (vector{5, 5}) || got.Expression.Name != "normal" || len(robot.images) != 23 {
		t.Fatalf("初始状态不正确：%+v", got)
	}
}

func TestDownloadedResourcesMatchExpressions(t *testing.T) {
	gifFiles, err := filepath.Glob("ui/gif/*.gif")
	if err != nil {
		t.Fatal(err)
	}
	if len(gifFiles) != len(expressions) {
		t.Fatalf("有 %d 张 GIF，登记了 %d 张", len(gifFiles), len(expressions))
	}
	for _, item := range expressions {
		if _, err := os.Stat(filepath.Join("ui", "gif", item.File)); err != nil {
			t.Errorf("表情 %s 的素材不存在：%v", item.Name, err)
		}
	}
}
