//go:build windows

package window

import (
	"errors"
	"image"
	"testing"
)

type emptyFrameSource struct{}

func (emptyFrameSource) Frame(int, int) (*image.RGBA, error) { return nil, nil }

type updatingFrameSource struct {
	emptyFrameSource
	called int
	err    error
}

func (source *updatingFrameSource) Update() error {
	source.called++
	return source.err
}

func TestPreviewProcessesQueuedUpdates(t *testing.T) {
	source := &updatingFrameSource{}
	game := &preview{source: source}
	if err := game.Update(); err != nil {
		t.Fatal(err)
	}
	if source.called != 1 {
		t.Fatalf("主循环执行了 %d 次更新，期望 1 次", source.called)
	}
	want := errors.New("更新失败")
	source.err = want
	if err := game.Update(); !errors.Is(err, want) {
		t.Fatalf("Update = %v，期望 %v", err, want)
	}
	if err := game.Update(); !errors.Is(err, want) || source.called != 2 {
		t.Fatal("更新出错后应保持原错误，不继续执行排队任务")
	}
}

func TestPreviewStopsAfterWindowCloseRequest(t *testing.T) {
	window := &presenter{}
	source := &updatingFrameSource{}
	game := &preview{source: source, closeRequested: &window.closeRequested}
	if err := window.Close(); err != nil {
		t.Fatal(err)
	}
	if err := game.Update(); !errors.Is(err, errWindowClosed) || source.called != 0 {
		t.Fatal("关闭窗口后应直接退出，不再处理排队更新")
	}
}
