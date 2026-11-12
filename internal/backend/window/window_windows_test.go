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
