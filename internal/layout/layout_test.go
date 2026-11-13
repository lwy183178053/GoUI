package layout

import (
	"image"
	"testing"

	"github.com/lwy183178053/GoUI/internal/ui"
)

func TestIndependentAxes(t *testing.T) {
	tests := []struct {
		name   string
		size   image.Point
		margin ui.Insets
		want   image.Rectangle
	}{
		{"fixed ignores all margins", image.Pt(40, 20), ui.Insets{Left: 10, Top: 20, Right: 30, Bottom: 40}, image.Rect(33, 38, 73, 58)},
		{"width fill ignores x anchor pivot position", image.Pt(-1, 20), ui.Insets{Left: 10, Top: 20, Right: 30, Bottom: 40}, image.Rect(10, 38, 70, 58)},
		{"height fill ignores y anchor pivot position", image.Pt(40, -1), ui.Insets{Left: 10, Top: 20, Right: 30, Bottom: 40}, image.Rect(33, 20, 73, 60)},
		{"both fill", image.Pt(-1, -1), ui.Insets{Left: 10, Top: 20, Right: 30, Bottom: 40}, image.Rect(10, 20, 70, 60)},
		{"oversized margins", image.Pt(-1, -1), ui.Insets{Left: 70, Top: 90, Right: 50, Bottom: 20}, image.Rect(70, 90, 70, 90)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			child := &ui.Node{Kind: ui.Block, BlockStyle: ui.BlockStyle{Size: tt.size, Margin: tt.margin, Anchor: ui.Vec2{X: 0.5, Y: 0.5}, Pivot: ui.Vec2{X: 0.5, Y: 0.5}, Position: image.Pt(3, -2)}}
			box := Compute(&ui.Node{Kind: ui.UI, Children: []*ui.Node{child}}, 100, 100).Children[0]
			if box.Pixels != tt.want {
				t.Fatalf("got %v, want %v", box.Pixels, tt.want)
			}
		})
	}
}

func TestFractionalGeometryAndResizing(t *testing.T) {
	child := &ui.Node{Kind: ui.Block, BlockStyle: ui.BlockStyle{Size: image.Pt(240, 80), Anchor: ui.Vec2{X: 0.5, Y: 0.5}, Pivot: ui.Vec2{X: 0.5, Y: 0.5}}}
	grandchild := &ui.Node{Kind: ui.Block, BlockStyle: ui.BlockStyle{Size: image.Pt(1, 1), Anchor: ui.Vec2{X: 0.5}}}
	child.Children = []*ui.Node{grandchild}
	root := &ui.Node{Kind: ui.UI, Children: []*ui.Node{child}}
	first := Compute(root, 801, 400)
	box := first.Children[0]
	if box.Geometry.X != 280.5 || box.Pixels != image.Rect(281, 160, 521, 240) {
		t.Fatalf("wrong half-pixel layout: %#v", box)
	}
	if got := box.Children[0]; got.Geometry.X != 400.5 || got.Pixels.Min.X != 401 {
		t.Fatalf("descendants must use fractional parent geometry: %#v", got)
	}
	second := Compute(root, 1000, 600)
	if got := second.Children[0].Pixels; got != image.Rect(380, 260, 620, 340) {
		t.Fatalf("resized bounds = %v", got)
	}
	if first.Pixels != image.Rect(0, 0, 801, 400) {
		t.Fatal("computing new geometry mutated an old layout")
	}
}

func TestNegativeHalfRoundsAwayFromZero(t *testing.T) {
	child := &ui.Node{Kind: ui.Block, BlockStyle: ui.BlockStyle{Size: image.Pt(1, 1), Pivot: ui.Vec2{X: 0.5, Y: 0.5}}}
	box := Compute(&ui.Node{Kind: ui.UI, Children: []*ui.Node{child}}, 10, 10).Children[0]
	if box.Pixels != image.Rect(-1, -1, 1, 1) {
		t.Fatalf("edges must round independently away from zero: %v", box.Pixels)
	}
}

func TestNestedPositionAndSharedEdges(t *testing.T) {
	parent := &ui.Node{Kind: ui.Block, BlockStyle: ui.BlockStyle{Size: image.Pt(10, 10), Anchor: ui.Vec2{X: 0.5}, Pivot: ui.Vec2{X: 0.5}}}
	left := &ui.Node{Kind: ui.Block, BlockStyle: ui.BlockStyle{Size: image.Pt(5, 5)}}
	right := &ui.Node{Kind: ui.Block, BlockStyle: ui.BlockStyle{Size: image.Pt(5, 5), Position: image.Pt(5, 0)}}
	parent.Children = []*ui.Node{left, right}
	box := Compute(&ui.Node{Kind: ui.UI, Children: []*ui.Node{parent}}, 101, 30).Children[0]
	if box.Children[0].Pixels.Max.X != box.Children[1].Pixels.Min.X || box.Children[1].Geometry.X != 50.5 {
		t.Fatalf("shared edges separated or fractional parent lost: %#v", box)
	}
}
