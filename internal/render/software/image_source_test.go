package software

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lwy183178053/GoUI/internal/ui"
	"golang.org/x/image/font/gofont/goregular"
)

func sourceRenderer(t *testing.T) (*Renderer, *ui.Document) {
	t.Helper()
	palette := color.Palette{color.Black, color.White}
	one := image.NewPaletted(image.Rect(0, 0, 1, 1), palette)
	one.SetColorIndex(0, 0, 1)
	var gifData, pngData bytes.Buffer
	if err := gif.EncodeAll(&gifData, &gif.GIF{Image: []*image.Paletted{one, one}, Delay: []int{10, 10}, LoopCount: 0}); err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(&pngData, one); err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{
		"config.xml": {Data: []byte(`<config><fonts><font name="default" src="font.ttf"/></fonts></config>`)},
		"font.ttf":   {Data: goregular.TTF},
		"main.xml": {Data: []byte(`<ui><image id="face" size="4,4" src="first_[0.1s].gif" fit="contain" playback="segmented">
  <block id="overlay" size="1,1" color="#FF0000"/>
  <text id="answer" size="0,0">回答</text>
</image></ui>`)},
		"first_[0.1s].gif": {Data: gifData.Bytes()},
		"next_[0.1s].gif":  {Data: gifData.Bytes()},
		"static.png":       {Data: pngData.Bytes()},
		"broken.gif":       {Data: []byte("not an image")},
	}
	doc, err := ui.LoadFS(files)
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := New(doc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = renderer.Close() })
	return renderer, doc
}

func TestImageSourceLoadsOnlySelectedResourceAndKeepsChildren(t *testing.T) {
	renderer, doc := sourceRenderer(t)
	for _, resource := range doc.Resources {
		if resource == "next_[0.1s].gif" || resource == "broken.gif" {
			t.Fatal("启动时不应加载候选素材")
		}
	}
	frameFor(t, renderer, 8, 8)
	old := renderer.imagePlayers[doc.Find("face")]
	called := false
	if err := renderer.FinishImage("face", func() error { called = true; return nil }); err != nil {
		t.Fatal(err)
	}
	children := doc.Find("face").Children
	if err := renderer.SetImageSource("face", "next_[0.1s].gif"); err != nil {
		t.Fatal(err)
	}
	node := doc.Find("face")
	if old.decoder != nil || old.exit != nil || len(renderer.imagePlayers) != 1 {
		t.Fatal("旧 GIF 的画布、回调或播放器没有释放")
	}
	if node.Children[0] != children[0] || node.Children[1] != children[1] || node.Image.Fit != "contain" || doc.Find("answer").Text.Content != "回答" {
		t.Fatal("换源不应重建子节点或改变图片样式")
	}
	frame := frameFor(t, renderer, 8, 8)
	pixel(t, frame, 0, 0, color.RGBA{R: 255, A: 255})
	pixel(t, frame, 1, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	if called {
		t.Fatal("换源后不应执行旧回调")
	}
	current := renderer.imagePlayers[node]
	if err := renderer.SetImageSource("face", "next_[0.1s].gif"); err != nil {
		t.Fatal(err)
	}
	if renderer.imagePlayers[node] == current {
		t.Fatal("同一路径也应明确重新加载")
	}
}

func TestImageSourceFailureLeavesOldPlaybackAndTextUntouched(t *testing.T) {
	renderer, doc := sourceRenderer(t)
	frameFor(t, renderer, 8, 8)
	node := doc.Find("face")
	old, player := node.Image, renderer.imagePlayers[node]
	for _, src := range []string{"missing.gif", "broken.gif", "../outside.png", "static.png"} {
		if err := renderer.SetImageSource("face", src); err == nil || !strings.Contains(err.Error(), "main.xml:") {
			t.Fatalf("非法换源应报告 XML 位置：%s, %v", src, err)
		}
		if node.Image != old || renderer.imagePlayers[node] != player || doc.Find("answer").Text.Content != "回答" {
			t.Fatal("失败换源修改了当前节点")
		}
	}
	for _, id := range []string{"missing", "answer"} {
		if err := renderer.SetImageSource(id, "next_[0.1s].gif"); err == nil {
			t.Fatal("缺少 ID 或类型错误应报错")
		}
	}
}

func TestGIFSourceSwitchFromFinishCallbackDrawsNewFirstFrame(t *testing.T) {
	renderer, doc := sourceRenderer(t)
	frameFor(t, renderer, 8, 8)
	if err := renderer.FinishImage("face", func() error {
		return renderer.SetImageSource("face", "next_[0.1s].gif")
	}); err != nil {
		t.Fatal(err)
	}
	if err := renderer.advanceImageAnimations(renderer.layout, time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	player := renderer.imagePlayers[doc.Find("face")]
	if player.current != 0 || player.frame() == nil || doc.Find("face").Image.Source != "next_[0.1s].gif" {
		t.Fatal("回调换源时应在同一帧准备好新图片")
	}
}

func TestImageSourceCanSwitchBetweenStaticAndAnimated(t *testing.T) {
	renderer, doc := sourceRenderer(t)
	node := doc.Find("face")
	// auto 模式允许静态图和普通 GIF 互换，分段模式继续要求带参数的 GIF。
	node.Image.Playback = "auto"
	if err := renderer.SetImageSource("face", "static.png"); err != nil {
		t.Fatal(err)
	}
	if renderer.imagePlayers[node] != nil || node.Image.Image == nil {
		t.Fatal("静态图不应保留旧播放器")
	}
	frameFor(t, renderer, 8, 8)
	if err := renderer.SetImageSource("face", "next_[0.1s].gif"); err != nil {
		t.Fatal(err)
	}
	frameFor(t, renderer, 8, 8)
	if renderer.imagePlayers[node] == nil || node.Image.Animation == nil || node.Image.Animation.Segments != nil {
		t.Fatal("普通 GIF 换源未保留 auto 模式")
	}
}
