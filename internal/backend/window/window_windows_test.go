//go:build windows

package window

import (
	"errors"
	"image"
	"testing"
)

type emptyFrameSource struct{}

func (emptyFrameSource) Frame(int, int) (*image.RGBA, error) { return nil, nil }

func TestPreviewCallsReadyOnce(t *testing.T) {
	called := 0
	game := &preview{
		source: emptyFrameSource{},
		onReady: func() error {
			called++
			return nil
		},
	}
	if err := game.Update(); err != nil {
		t.Fatal(err)
	}
	if err := game.Update(); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("ready callback called %d times, want once", called)
	}
}

func TestPreviewStopsWhenReadyReturnsError(t *testing.T) {
	want := errors.New("page initialization failed")
	game := &preview{
		source:  emptyFrameSource{},
		onReady: func() error { return want },
	}
	if err := game.Update(); !errors.Is(err, want) {
		t.Fatalf("Update returned %v, want %v", err, want)
	}
	if err := game.Update(); !errors.Is(err, want) {
		t.Fatalf("second Update returned %v, want %v", err, want)
	}
}

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
}
