package window

import (
	"strings"
	"testing"
)

func TestValidateDisplayIndex(t *testing.T) {
	if err := validateDisplayIndex(0, 1); err != nil {
		t.Fatalf("primary display rejected: %v", err)
	}
	for _, index := range []int{-1, 1} {
		if err := validateDisplayIndex(index, 1); err == nil {
			t.Errorf("display index %d unexpectedly accepted", index)
		}
	}
	if err := validateDisplayIndex(0, 0); err == nil || !strings.Contains(err.Error(), "detected 0 display") {
		t.Fatalf("empty display list returned %v", err)
	}
}

func TestResolveWindowSize(t *testing.T) {
	width, height, err := resolveWindowSize(800, 600, 1920, 1080, false)
	if err != nil || width != 800 || height != 600 {
		t.Fatalf("window size = %dx%d, error = %v", width, height, err)
	}
	width, height, err = resolveWindowSize(800, 600, 1920, 1080, true)
	if err != nil || width != 1920 || height != 1080 {
		t.Fatalf("fullscreen size = %dx%d, error = %v", width, height, err)
	}
	if _, _, err := resolveWindowSize(800, 600, 0, 0, true); err == nil {
		t.Fatal("fullscreen accepted a display with no usable dimensions")
	}
}

func TestRenderSizeUsesDisplayScale(t *testing.T) {
	width, height := renderSize(2133, 1200, 1.2)
	if width != 2560 || height != 1440 {
		t.Fatalf("render size = %dx%d, want 2560x1440", width, height)
	}

	width, height = renderSize(800, 600, 1)
	if width != 800 || height != 600 {
		t.Fatalf("render size at 100%% = %dx%d, want 800x600", width, height)
	}
}

func TestWindowControls(t *testing.T) {
	presenter := New(Options{}).(*presenter)
	if err := presenter.ToggleMaximize(); err == nil || !strings.Contains(err.Error(), "resizable is false") {
		t.Fatalf("non-resizable maximize returned %v", err)
	}
	if err := presenter.Close(); err != nil {
		t.Fatal(err)
	}
	if !presenter.closeRequested.Load() {
		t.Fatal("close request was not recorded")
	}
}
