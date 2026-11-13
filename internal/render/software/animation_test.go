package software

import (
	"image"
	"image/color"
	"math"
	"testing"
	"time"

	"github.com/lwy183178053/GoUI/internal/ui"
)

func TestTweenInterpolatesLinearly(t *testing.T) {
	start := time.Unix(100, 0)
	animation := tween{
		from: [2]float64{2, -4}, to: [2]float64{12, 6},
		started: start, duration: time.Second,
	}
	value, running := animation.sample(start.Add(500 * time.Millisecond))
	if !running || value != [2]float64{7, 1} {
		t.Fatalf("halfway tween = %v, running=%v", value, running)
	}
	value, running = animation.sample(start.Add(time.Second))
	if running || value != animation.to {
		t.Fatalf("completed tween = %v, running=%v", value, running)
	}
}

func TestAnimationDoesNotRepublishAnUnchangedFrame(t *testing.T) {
	item := block(0, 0, 4, 4, red)
	item.ID = "item"
	renderer := rendererFor(t, item)
	first := frameFor(t, renderer, 8, 8)

	started := time.Now().Add(time.Hour)
	renderer.motions["item"] = &animation{
		position: [2]float64{0, 0},
		scale:    [2]float64{1, 1},
		rotate: &tween{
			from: [2]float64{0}, to: [2]float64{90},
			started: started, duration: time.Second,
		},
	}

	frame, err := renderer.Frame(8, 8)
	if err != nil {
		t.Fatal(err)
	}
	if frame != first {
		t.Fatal("unchanged animation frame was rendered again")
	}
}

func TestScaleAnimationChangesDrawingAndButtonHit(t *testing.T) {
	button := block(2, 2, 4, 4, red)
	button.ID = "button"
	button.Kind = ui.Button
	button.Button = &ui.ButtonStyle{}
	renderer := rendererFor(t, button)
	frameFor(t, renderer, 16, 16)
	if err := renderer.ScaleTo("button", 2, 2, 0); err != nil {
		t.Fatal(err)
	}
	frame := frameFor(t, renderer, 16, 16)
	pixel(t, frame, 8, 8, color.RGBA{R: 255, A: 255})

	clicked := false
	if err := renderer.SetButtonEvent("button", ui.Click, func() error {
		clicked = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := renderer.PointerDown(8, 8); err != nil {
		t.Fatal(err)
	}
	if err := renderer.PointerUp(8, 8); err != nil {
		t.Fatal(err)
	}
	if !clicked {
		t.Fatal("scaled button was drawn but could not be clicked")
	}
}

func TestRotationAnimationChangesDrawingAndButtonHit(t *testing.T) {
	button := block(5, 5, 8, 4, red)
	button.ID = "button"
	button.Kind = ui.Button
	button.Pivot = ui.Vec2{X: 0.5, Y: 0.5}
	button.Button = &ui.ButtonStyle{}
	renderer := rendererFor(t, button)
	frameFor(t, renderer, 20, 16)
	if err := renderer.RotateTo("button", 90, 0); err != nil {
		t.Fatal(err)
	}
	frame := frameFor(t, renderer, 20, 16)
	pixel(t, frame, 5, 2, color.RGBA{R: 255, A: 255})
	pixel(t, frame, 8, 5, black)

	clicked := false
	if err := renderer.SetButtonEvent("button", ui.Click, func() error {
		clicked = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := renderer.PointerDown(5, 2); err != nil {
		t.Fatal(err)
	}
	if err := renderer.PointerUp(5, 2); err != nil {
		t.Fatal(err)
	}
	if !clicked {
		t.Fatal("rotated button was drawn but could not be clicked")
	}
}

func TestMoveAnimationUpdatesLayout(t *testing.T) {
	item := block(0, 0, 4, 4, red)
	item.ID = "item"
	renderer := rendererFor(t, item)
	first := frameFor(t, renderer, 16, 8)
	pixel(t, first, 1, 1, color.RGBA{R: 255, A: 255})
	if err := renderer.MoveTo("item", 8, 0, 0); err != nil {
		t.Fatal(err)
	}
	second := frameFor(t, renderer, 16, 8)
	pixel(t, second, 1, 1, black)
	pixel(t, second, 9, 1, color.RGBA{R: 255, A: 255})
}

func TestMoveAnimationRendersIntermediatePosition(t *testing.T) {
	item := block(0, 0, 4, 4, red)
	item.ID = "item"
	renderer := rendererFor(t, item)
	frameFor(t, renderer, 16, 8)
	started := time.Now().Add(-500 * time.Millisecond)
	renderer.motions["item"] = &animation{
		position: [2]float64{0, 0},
		scale:    [2]float64{1, 1},
		move: &tween{
			from: [2]float64{0, 0}, to: [2]float64{8, 0},
			started: started, duration: time.Second,
		},
	}
	frame := frameFor(t, renderer, 16, 8)
	ink := inkBounds(frame)
	if ink.Min.X < 3 || ink.Min.X > 5 || ink.Max.X < 7 || ink.Max.X > 9 {
		t.Fatalf("halfway movement rendered at %v, want a 4-pixel offset", ink)
	}
}

func TestAnimationRejectsInvalidTargets(t *testing.T) {
	item := block(0, 0, 4, 4, red)
	item.ID = "item"
	renderer := rendererFor(t, item)
	if err := renderer.MoveTo("item", 1, 1, -time.Millisecond); err == nil {
		t.Fatal("negative animation duration unexpectedly succeeded")
	}
	if err := renderer.RotateTo("item", math.Inf(1), 0); err == nil {
		t.Fatal("infinite rotation unexpectedly succeeded")
	}
	if err := renderer.ScaleTo("item", 0, 1, 0); err == nil {
		t.Fatal("zero scale unexpectedly succeeded")
	}
}

func TestMoveAnimationRespectsFilledAxes(t *testing.T) {
	tests := []struct {
		name string
		size image.Point
	}{
		{"横向填充", image.Pt(-1, 4)},
		{"纵向填充", image.Pt(4, -1)},
		{"全部填充", image.Pt(-1, -1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := block(0, 0, tt.size.X, tt.size.Y, red)
			item.ID = "item"
			renderer := rendererFor(t, item)
			frameFor(t, renderer, 40, 40)
			initial := renderer.boxes["item"].Pixels
			if err := renderer.MoveTo("item", 20, 24, time.Second); err != nil {
				t.Fatal(err)
			}
			started := renderer.motions["item"].move.started
			for _, elapsed := range []time.Duration{500 * time.Millisecond, time.Second} {
				renderer.advanceAnimations(started.Add(elapsed))
				box := renderer.boxes["item"]
				transform := nodeTransform(box, renderer.motions["item"])
				moved := translatedBox(box, transform[4], transform[5])
				offset := image.Pt(10, 12)
				if elapsed == time.Second {
					offset = image.Pt(20, 24)
				}
				if tt.size.X == -1 {
					offset.X = 0
				}
				if tt.size.Y == -1 {
					offset.Y = 0
				}
				if want := initial.Add(offset); moved.Pixels != want {
					t.Fatalf("%v 后矩形 = %v，期望 %v", elapsed, moved.Pixels, want)
				}
			}
		})
	}
}
