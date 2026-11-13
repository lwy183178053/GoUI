package goui

import (
	"errors"
	"testing"

	"github.com/lwy183178053/GoUI/internal/ui"
)

func TestImageFinishForwardsPageAndCallbackError(t *testing.T) {
	controls := &testPageControls{kinds: map[string]ui.Kind{"gif": ui.Image}}
	page := &Page{renderer: controls}
	image, err := page.GetImage("gif")
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("完成回调失败")
	called := false
	if err := image.Finish(func(got *Page) error {
		called = got == page
		return failure
	}); err != nil {
		t.Fatal(err)
	}
	if called || controls.finish == nil {
		t.Fatal("Finish 应登记回调，而不是立即执行")
	}
	if err := controls.finish(); !errors.Is(err, failure) || !called {
		t.Fatalf("完成回调没有收到 Page 或原始错误：%v", err)
	}
	if err := image.Finish(nil); err == nil {
		t.Fatal("缺少完成回调应报错")
	}
	if err := (*Image)(nil).Finish(func(*Page) error { return nil }); err == nil {
		t.Fatal("空 Image 应报错")
	}
	if err := (&Image{}).Finish(func(*Page) error { return nil }); err == nil {
		t.Fatal("未连接的 Image 应报错")
	}
}

func TestImageSetSourceForwardsAndRejectsDisconnectedImage(t *testing.T) {
	controls := &testPageControls{kinds: map[string]ui.Kind{"picture": ui.Image}}
	page := &Page{renderer: controls}
	picture, err := page.GetImage("picture")
	if err != nil {
		t.Fatal(err)
	}
	if err := picture.SetSource("gif/happy.gif"); err != nil {
		t.Fatal(err)
	}
	if controls.imageSource != "gif/happy.gif" {
		t.Fatal("图片路径未转交给渲染器")
	}
	if err := (*Image)(nil).SetSource("a.png"); err == nil {
		t.Fatal("空 Image 应报错")
	}
	if err := (&Image{}).SetSource("a.png"); err == nil {
		t.Fatal("未连接的 Image 应报错")
	}
}

func TestImagePlayForwardsPageAndCallbackError(t *testing.T) {
	controls := &testPageControls{kinds: map[string]ui.Kind{"gif": ui.Image}}
	page := &Page{renderer: controls}
	image, err := page.GetImage("gif")
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("完成回调失败")
	called := false
	if err := image.Play(false, func(got *Page) error {
		called = got == page
		return failure
	}); err != nil {
		t.Fatal(err)
	}
	if called || controls.playLoop || controls.play == nil {
		t.Fatal("Play 应登记回调而非立即调用")
	}
	if err := controls.play(); !errors.Is(err, failure) || !called {
		t.Fatalf("Play 完成通知未传递 Page 或错误：%v", err)
	}
	if err := image.Play(true, nil); err != nil || !controls.playLoop || controls.play != nil {
		t.Fatalf("循环控制未转交：%v", err)
	}
	for _, disconnected := range []*Image{nil, {}} {
		if err := disconnected.Play(false, nil); err == nil {
			t.Fatal("未连接的 Image 不能 Play")
		}
	}
}
