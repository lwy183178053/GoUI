package goui

import (
	"context"
	"errors"
	"image"
	"image/color"
	"testing"

	"github.com/lwy183178053/GoUI/internal/render/software"
	"github.com/lwy183178053/GoUI/internal/ui"
)

func TestApplicationSnapshotRunsThroughDispatch(t *testing.T) {
	app, source := dispatchTestApplication(t)
	renderer, err := software.New(&ui.Document{Root: &ui.Node{Kind: ui.UI, BlockStyle: ui.BlockStyle{Visible: true, Color: color.NRGBA{R: 200, A: 255}}}})
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Close()
	app.renderer = renderer
	if _, err := renderer.Frame(8, 4); err != nil {
		t.Fatal(err)
	}
	type result struct {
		frame *image.RGBA
		err   error
	}
	done := make(chan result, 1)
	go func() { frame, err := app.Snapshot(context.Background()); done <- result{frame, err} }()
	request := takeUpdate(t, app)
	app.updates <- request
	if err := source.Update(); err != nil {
		t.Fatal(err)
	}
	got := <-done
	if got.err != nil || got.frame.Bounds().Size() != image.Pt(8, 4) || got.frame.RGBAAt(0, 0).R != 200 {
		t.Fatalf("截图未通过主循环返回正确画面: %v", got.err)
	}
}

func TestApplicationSnapshotRequiresRunningWindow(t *testing.T) {
	for _, app := range []*Application{nil, {}} {
		if _, err := app.Snapshot(context.Background()); err == nil {
			t.Fatal("未运行的窗口应返回错误")
		}
	}
}

func TestApplicationSnapshotHonorsCanceledContext(t *testing.T) {
	app, _ := dispatchTestApplication(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	frame, err := app.Snapshot(ctx)
	if frame != nil || !errors.Is(err, context.Canceled) || len(app.updates) != 0 {
		t.Fatalf("取消的截图不应排入主循环: %v", err)
	}
}
