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
