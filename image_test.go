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
