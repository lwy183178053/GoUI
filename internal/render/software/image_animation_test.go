package software

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/lwy183178053/GoUI/internal/ui"
)

func TestImagePlaybackUsesGIFDelaysAndLoops(t *testing.T) {
	player := newImagePlayback(testGIFAnimation(t, 0, 100*time.Millisecond, 200*time.Millisecond))
	if player.duration != 300*time.Millisecond {
		t.Fatalf("loop duration = %s, want 300ms", player.duration)
	}
	if got := player.frameAt(99 * time.Millisecond); got != 0 {
		t.Fatalf("frame at 99ms = %d, want 0", got)
	}
	if got := player.frameAt(100 * time.Millisecond); got != 1 {
		t.Fatalf("frame at 100ms = %d, want 1", got)
	}
	if got := player.frameAt(300 * time.Millisecond); got != 0 {
		t.Fatalf("frame at loop boundary = %d, want 0", got)
	}
}

func TestImagePlaybackHonorsFiniteLoopCount(t *testing.T) {
	player := newImagePlayback(testGIFAnimation(t, 1, 100*time.Millisecond, 100*time.Millisecond))
	if got := player.frameAt(300 * time.Millisecond); got != 1 {
		t.Fatalf("frame after one restart = %d, want final frame 1", got)
	}
	player = newImagePlayback(testGIFAnimation(t, -1, 100*time.Millisecond, 100*time.Millisecond))
	if got := player.frameAt(200 * time.Millisecond); got != 1 {
		t.Fatalf("frame after one-shot playback = %d, want final frame 1", got)
	}
}

func TestImagePlaybackInvalidatesOnlyWhenFrameChanges(t *testing.T) {
	start := time.Unix(100, 0)
	player := newImagePlayback(testGIFAnimation(t, 0, 50*time.Millisecond, 50*time.Millisecond))
	changed, err := player.advance(start)
	if err != nil || !changed || player.current != 0 {
		t.Fatal("first frame was not selected")
	}
	changed, err = player.advance(start.Add(20 * time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("same frame unexpectedly reported a change")
	}
	changed, err = player.advance(start.Add(50 * time.Millisecond))
	if err != nil || !changed || player.current != 1 {
		t.Fatal("second frame change was not detected")
	}
}

func testGIFAnimation(t *testing.T, loopCount int, delays ...time.Duration) *ui.GIFAnimation {
	t.Helper()
	animation := &ui.GIFAnimation{
		Width: 1, Height: 1, LoopCount: loopCount, Background: color.NRGBA{},
		Frames: make([]ui.GIFFrame, len(delays)),
	}
	for i, delay := range delays {
		var encoded bytes.Buffer
		pixel := image.NewRGBA(image.Rect(0, 0, 1, 1))
		pixel.Set(0, 0, color.NRGBA{R: uint8(i + 1), A: 255})
		if err := png.Encode(&encoded, pixel); err != nil {
			t.Fatal(err)
		}
		animation.Frames[i] = ui.GIFFrame{PNG: encoded.Bytes(), Bounds: image.Rect(0, 0, 1, 1), Delay: delay}
	}
	return animation
}

// 图片不写 ID 也要刷新自己的矩形；其它区域和同帧缓存不受影响。
func TestGIFFrameUpdatesAnonymousImageAndDirtyRegion(t *testing.T) {
	node := gifImageNode(t)
	node.Position = image.Pt(2, 3)
	renderer := rendererFor(t, node)
	first := frameFor(t, renderer, 8, 8)
	pixel(t, first, 2, 3, color.RGBA{R: 1, A: 255})
	player := renderer.imagePlayers[node]
	player.started = time.Now().Add(-90 * time.Minute)
	if err := renderer.advanceImageAnimations(renderer.layout, time.Now()); err != nil {
		t.Fatal(err)
	}
	if renderer.dirty != image.Rect(2, 3, 4, 5) {
		t.Fatalf("GIF dirty region = %v, want only image bounds", renderer.dirty)
	}
	second := frameFor(t, renderer, 8, 8)
	pixel(t, second, 2, 3, color.RGBA{R: 2, A: 255})
	pixel(t, second, 0, 0, black)
	if first == second {
		t.Fatal("changed GIF did not publish a new frame")
	}
	if third := frameFor(t, renderer, 8, 8); third != second {
		t.Fatal("unchanged GIF rasterized again")
	}
}

func TestHiddenGIFAndParentDoNotDecodeAndRestartOnShow(t *testing.T) {
	node := gifImageNode(t)
	node.ID = "gif"
	parent := block(0, 0, 4, 4, color.NRGBA{}, node)
	parent.ID, parent.Visible = "group", false
	renderer := rendererFor(t, parent)
	player := renderer.imagePlayers[node]
	first := frameFor(t, renderer, 8, 8)
	if player.decoder != nil || player.frame() != nil {
		t.Fatal("hidden parent decoded a child GIF")
	}
	pixel(t, first, 0, 0, black)
	if err := renderer.SetVisible("group", true); err != nil {
		t.Fatal(err)
	}
	frameFor(t, renderer, 8, 8)
	if player.current != 0 {
		t.Fatal("GIF did not start at first frame")
	}
	player.started = time.Now().Add(-90 * time.Minute)
	frameFor(t, renderer, 8, 8)
	if player.current != 1 {
		t.Fatal("GIF did not advance before hiding")
	}
	for _, id := range []string{"gif", "group"} {
		if err := renderer.SetVisible(id, false); err != nil {
			t.Fatal(err)
		}
		if player.decoder != nil || player.frame() != nil {
			t.Fatal("hidden GIF kept its playback canvas")
		}
		frameFor(t, renderer, 8, 8)
		if player.decoder != nil {
			t.Fatal("hidden GIF kept decoding")
		}
		if err := renderer.SetVisible(id, true); err != nil {
			t.Fatal(err)
		}
		frame := frameFor(t, renderer, 8, 8)
		if player.current != 0 {
			t.Fatal("shown GIF did not restart")
		}
		pixel(t, frame, 0, 0, color.RGBA{R: 1, A: 255})
	}
	player.started = time.Now().Add(-90 * time.Minute)
	frameFor(t, renderer, 8, 8)
	if err := renderer.SetVisible("gif", true); err != nil {
		t.Fatal(err)
	}
	frameFor(t, renderer, 8, 8)
	if player.current != 1 {
		t.Fatal("SetVisible(true) restarted an already visible GIF")
	}
}

func TestGIFSwitchClearsOldPixelsWithoutBlending(t *testing.T) {
	old := gifImageNode(t)
	old.ID, old.Size = "old", image.Pt(4, 2)
	next := gifImageNode(t)
	next.ID, next.Visible = "next", false
	renderer := rendererFor(t, old, next)
	frameFor(t, renderer, 8, 8)
	if err := renderer.SetVisible("old", false); err != nil {
		t.Fatal(err)
	}
	if err := renderer.SetVisible("next", true); err != nil {
		t.Fatal(err)
	}
	frame := frameFor(t, renderer, 8, 8)
	pixel(t, frame, 0, 0, color.RGBA{R: 1, A: 255})
	pixel(t, frame, 3, 0, black)
	if renderer.imagePlayers[old].decoder != nil {
		t.Fatal("old expression still playing")
	}
}

func TestGIFAnimationInvalidatesTransformedLayer(t *testing.T) {
	node := gifImageNode(t)
	node.ID = "gif"
	renderer := rendererFor(t, node)
	if err := renderer.ScaleTo("gif", 2, 2, 0); err != nil {
		t.Fatal(err)
	}
	first := frameFor(t, renderer, 8, 8)
	pixel(t, first, 1, 1, color.RGBA{R: 1, A: 255})
	layer := renderer.layers[node]
	renderer.imagePlayers[node].started = time.Now().Add(-90 * time.Minute)
	second := frameFor(t, renderer, 8, 8)
	pixel(t, second, 1, 1, color.RGBA{R: 2, A: 255})
	if layer == renderer.layers[node] {
		t.Fatal("GIF reused a transformed layer with old frame pixels")
	}
}

func gifImageNode(t *testing.T) *ui.Node {
	t.Helper()
	return &ui.Node{
		Kind: ui.Image, BlockStyle: ui.BlockStyle{Visible: true, Size: image.Pt(2, 2)},
		Image: &ui.ImageStyle{Fit: "contain", Animation: testGIFAnimation(t, 0, time.Hour, time.Hour)},
	}
}
