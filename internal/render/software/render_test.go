package software

import (
	"image"
	"image/color"
	"path/filepath"
	"reflect"
	"testing"

	"goui/internal/ui"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

var (
	red   = color.NRGBA{R: 255, A: 255}
	green = color.NRGBA{G: 255, A: 255}
	blue  = color.NRGBA{B: 255, A: 255}
	black = color.RGBA{A: 255}
)

func block(x, y, width, height int, fill color.NRGBA, children ...*ui.Node) *ui.Node {
	return &ui.Node{Kind: ui.Block, BlockStyle: ui.BlockStyle{Visible: true, Position: image.Pt(x, y), Size: image.Pt(width, height), Color: fill}, Children: children}
}

func textNode(x, y, width, height int, content string) *ui.Node {
	node := block(x, y, width, height, color.NRGBA{})
	node.Kind = ui.Text
	node.Text = &ui.TextStyle{Content: content, Font: "default", FontSize: 16, Color: red, Align: "left", VAlign: "top"}
	return node
}

func rendererFor(t *testing.T, children ...*ui.Node) *Renderer {
	t.Helper()
	fonts := make(map[string]*opentype.Font)
	for name, data := range map[string][]byte{"default": goregular.TTF, "bold": gobold.TTF} {
		parsed, err := opentype.Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		fonts[name] = parsed
	}
	renderer, err := New(&ui.Document{Root: &ui.Node{Kind: ui.UI, BlockStyle: ui.BlockStyle{Visible: true, Color: color.NRGBA{A: 255}}, Children: children}, Fonts: fonts})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := renderer.Close(); err != nil {
			t.Error(err)
		}
	})
	return renderer
}

func frameFor(t *testing.T, renderer *Renderer, width, height int) *image.RGBA {
	t.Helper()
	frame, err := renderer.Frame(width, height)
	if err != nil {
		t.Fatal(err)
	}
	return frame
}

func pixel(t *testing.T, frame *image.RGBA, x, y int, want color.RGBA) {
	t.Helper()
	if got := frame.RGBAAt(x, y); got != want {
		t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, got, want)
	}
}

func TestBackgroundAlphaAndPaintOrder(t *testing.T) {
	parent := block(0, 0, 8, 8, color.NRGBA{R: 255, A: 128}, block(0, 0, 6, 6, color.NRGBA{}))
	overlay := block(2, 2, 8, 8, color.NRGBA{B: 255, A: 128})
	frame := frameFor(t, rendererFor(t, parent, overlay), 12, 12)
	pixel(t, frame, 11, 11, black)
	pixel(t, frame, 1, 1, color.RGBA{R: 128, A: 255}) // 子节点不继承父节点颜色
	pixel(t, frame, 3, 3, color.RGBA{R: 63, B: 128, A: 255})
	pixel(t, frame, 9, 9, color.RGBA{B: 128, A: 255})
}

func TestClearColorPreservesPremultipliedAlpha(t *testing.T) {
	renderer, err := New(&ui.Document{Root: &ui.Node{Kind: ui.UI, BlockStyle: ui.BlockStyle{Visible: true, Color: color.NRGBA{R: 128, G: 64, B: 32, A: 128}}}})
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Close()
	frame := frameFor(t, renderer, 2, 2)
	pixel(t, frame, 0, 0, color.RGBA{R: 64, G: 32, B: 16, A: 128})
}
func TestParentDoesNotClipDescendants(t *testing.T) {
	parent := block(4, 4, 8, 8, green, block(-8, -8, 10, 10, red), block(10, 10, 20, 20, blue))
	parent.Radius = 4
	zero := block(20, 0, 0, 0, green, block(0, 0, 4, 4, red))
	frame := frameFor(t, rendererFor(t, parent, zero), 24, 24)
	pixel(t, frame, 0, 0, color.RGBA{R: 255, A: 255}) // 子节点可超出父节点，但不能超出画布
	pixel(t, frame, 4, 4, color.RGBA{R: 255, A: 255}) // 父节点的圆角不裁剪子节点
	pixel(t, frame, 23, 23, color.RGBA{B: 255, A: 255})
	pixel(t, frame, 21, 1, color.RGBA{R: 255, A: 255}) // 父节点尺寸为零时，子节点仍可见
}

func TestVisibilityHidesWholeSubtree(t *testing.T) {
	hidden := block(0, 0, 10, 10, red, block(12, 12, 5, 5, blue))
	hidden.Visible = false
	frame := frameFor(t, rendererFor(t, hidden), 20, 20)
	pixel(t, frame, 5, 5, black)
	pixel(t, frame, 14, 14, black)
}

func TestRoundedCornersAndRadiusClamp(t *testing.T) {
	first := block(0, 0, 16, 16, red)
	first.Radius = 8
	second := block(16, 0, 16, 16, red)
	second.Radius = 999
	frame := frameFor(t, rendererFor(t, first, second), 32, 16)
	pixel(t, frame, 0, 0, black)
	pixel(t, frame, 8, 8, color.RGBA{R: 255, A: 255})
	antialias := false
	for y := range 16 {
		for x := range 16 {
			a, b := frame.RGBAAt(x, y), frame.RGBAAt(x+16, y)
			if a != b {
				t.Fatalf("radius should clamp to half the short side at (%d,%d): %v != %v", x, y, a, b)
			}
			antialias = antialias || (a.R > 0 && a.R < 255)
		}
	}
	if !antialias {
		t.Fatal("rounded edges have no partial coverage")
	}
}

func TestFrameCacheAndResize(t *testing.T) {
	child := block(0, 0, 10, 10, red)
	child.Anchor = ui.Vec2{X: 1, Y: 1}
	child.Pivot = ui.Vec2{X: 1, Y: 1}
	renderer := rendererFor(t, child)
	first := frameFor(t, renderer, 100, 100)
	if same := frameFor(t, renderer, 100, 100); same != first {
		t.Fatal("unchanged static frame must be reused")
	}
	second := frameFor(t, renderer, 120, 80)
	if second == first || second.Rect != image.Rect(0, 0, 120, 80) {
		t.Fatal("resizing must produce a new correctly sized frame")
	}
	pixel(t, second, 115, 75, color.RGBA{R: 255, A: 255})
	pixel(t, second, 95, 75, black)
	pixel(t, first, 95, 95, color.RGBA{R: 255, A: 255})
	if _, err := renderer.Frame(0, 100); err == nil {
		t.Fatal("invalid render sizes must be rejected")
	}
	if err := renderer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := renderer.Frame(1, 1); err == nil {
		t.Fatal("closed renderer must reject rendering")
	}
}

func TestTextLineBreaking(t *testing.T) {
	tests := []struct {
		name, content string
		width         int
		wrap          bool
		want          []string
	}{
		{"single line", "ABC DEF\nGHI", 7, false, []string{"ABC DEF GHI"}},
		{"word boundary", "ABC DEF", 28, true, []string{"ABC", "DEF"}},
		{"long word", "ABCDE", 14, true, []string{"AB", "CD", "E"}},
		{"Chinese characters", "中文界面", 14, true, []string{"中文", "界面"}},
		{"paragraph breaks", "ABC\n\nDEF", 28, true, []string{"ABC", "", "DEF"}},
		{"too narrow", "ABC", 1, true, []string{"A", "B", "C"}},
		{"blank", "", 28, true, []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := textLines(tt.content, basicfont.Face7x13, tt.width, tt.wrap)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func inkBounds(frame *image.RGBA) image.Rectangle {
	var result image.Rectangle
	for y := frame.Rect.Min.Y; y < frame.Rect.Max.Y; y++ {
		for x := frame.Rect.Min.X; x < frame.Rect.Max.X; x++ {
			if frame.RGBAAt(x, y).R != 0 {
				result = result.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return result
}

func TestTextAlignment(t *testing.T) {
	// 固定 Go Regular 字体在 16px 下，"MMMM" 宽 52px、行高 19px；
	// 使用 59px 宽度可让各对齐方式都落在整数像素上。
	var first image.Rectangle
	for _, align := range []string{"left", "center", "right"} {
		for _, valign := range []string{"top", "middle", "bottom"} {
			t.Run(align+"/"+valign, func(t *testing.T) {
				node := textNode(0, 0, 100, 59, "MMMM")
				node.Text.Align, node.Text.VAlign = align, valign
				renderer := rendererFor(t, node)
				frame := frameFor(t, renderer, 100, 59)
				got := inkBounds(frame)
				if got.Empty() {
					t.Fatal("font produced no visible pixels")
				}
				if first.Empty() {
					first = got
				}
				face := renderer.faces[faceKey{"default", 16}]
				freeX := 100*64 - int(font.MeasureString(face, "MMMM"))
				freeY := 59*64 - int(face.Metrics().Height)
				x, y := 0, 0
				if align == "center" {
					x = freeX / 2 / 64
				} else if align == "right" {
					x = freeX / 64
				}
				if valign == "middle" {
					y = freeY / 2 / 64
				} else if valign == "bottom" {
					y = freeY / 64
				}
				if got != first.Add(image.Pt(x, y)) {
					t.Fatalf("glyph bounds = %v, want %v", got, first.Add(image.Pt(x, y)))
				}
			})
		}
	}
}

func TestTextClipsToOwnRectangle(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		node := textNode(10, 10, 12, 12, "MMMMMMMMMMMMMMMM")
		node.Text.Wrap = wrap
		node.Text.FontSize = 24
		frame := frameFor(t, rendererFor(t, node), 40, 40)
		ink := inkBounds(frame)
		if ink.Empty() || !ink.In(image.Rect(10, 10, 22, 22)) {
			t.Fatalf("wrap=%v: text pixels escaped or disappeared: %v", wrap, ink)
		}
	}
}

func TestTextUsesSelectedFontAndOwnColor(t *testing.T) {
	regular := textNode(0, 0, 100, 30, "Preview")
	bold := textNode(0, 0, 100, 30, "Preview")
	bold.Text.Font = "bold"
	a := frameFor(t, rendererFor(t, regular), 100, 30)
	b := frameFor(t, rendererFor(t, bold), 100, 30)
	if reflect.DeepEqual(a.Pix, b.Pix) {
		t.Fatal("selecting a different registered font did not affect glyphs")
	}
	regular.Color = blue
	frame := frameFor(t, rendererFor(t, regular), 100, 30)
	pixel(t, frame, 99, 29, color.RGBA{B: 255, A: 255})
	if inkBounds(frame).Empty() {
		t.Fatal("block background replaced text-color")
	}
}

func TestExistingUIRendersAndResizes(t *testing.T) {
	doc, err := ui.Load(filepath.Join("..", "..", "..", "GoUI"))
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := New(doc)
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Close()
	for _, size := range []image.Point{image.Pt(960, 640), image.Pt(801, 480)} {
		frame := frameFor(t, renderer, size.X, size.Y)
		if frame.Bounds() != image.Rect(0, 0, size.X, size.Y) || len(frame.Pix) != size.X*size.Y*4 {
			t.Fatalf("existing UI produced an invalid frame at %v", size)
		}
	}
}

func TestImageFitModes(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for y := range 2 {
		for x := range 4 {
			if x < 2 {
				source.Set(x, y, red)
			} else {
				source.Set(x, y, blue)
			}
		}
	}
	makeImage := func(size image.Point, fit string) *ui.Node {
		return &ui.Node{Kind: ui.Image, BlockStyle: ui.BlockStyle{Visible: true, Size: size}, Image: &ui.ImageStyle{Image: source, Fit: fit}}
	}
	contain := frameFor(t, rendererFor(t, makeImage(image.Pt(4, 4), "contain")), 4, 4)
	if contain.RGBAAt(0, 0) != black || contain.RGBAAt(1, 1) == black {
		t.Fatal("contain should preserve the image ratio and leave transparent bands")
	}
	cover := frameFor(t, rendererFor(t, makeImage(image.Pt(4, 4), "cover")), 4, 4)
	for y := range 4 {
		for x := range 4 {
			if cover.RGBAAt(x, y).A != 255 {
				t.Fatalf("cover left a gap at (%d,%d)", x, y)
			}
		}
	}
	stretch := frameFor(t, rendererFor(t, makeImage(image.Pt(4, 4), "stretch")), 4, 4)
	if stretch.RGBAAt(0, 0).A != 255 || stretch.RGBAAt(3, 3).A != 255 {
		t.Fatal("stretch should fill the complete node rectangle")
	}
	rounded := makeImage(image.Pt(4, 4), "stretch")
	rounded.Radius = 2
	roundedFrame := frameFor(t, rendererFor(t, rounded), 4, 4)
	corner := roundedFrame.RGBAAt(0, 0)
	if corner.R == 0 || corner.R == 255 || roundedFrame.RGBAAt(1, 1) == black {
		t.Fatal("radius should antialias and clip the image corners")
	}
}

func TestButtonActionsAndDirtyUpdates(t *testing.T) {
	lower := block(0, 0, 20, 20, red)
	lower.ID = "run"
	lower.Kind = ui.Button
	lower.Button = &ui.ButtonStyle{Action: "preview.run"}
	upper := block(0, 0, 10, 20, green)
	upper.ID = "disabled"
	upper.Kind = ui.Button
	upper.Button = &ui.ButtonStyle{Action: "preview.disabled", Disabled: true}
	status := textNode(20, 0, 20, 20, "old")
	status.ID = "status"
	group := block(0, 22, 5, 5, green, block(8, 8, 4, 4, blue))
	group.ID = "group"
	outside := block(30, 30, 5, 5, red)
	renderer := rendererFor(t, lower, upper, status, group, outside)
	first := frameFor(t, renderer, 40, 40)
	called := 0
	if err := renderer.RegisterAction("preview.disabled", func() error { called += 10; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := renderer.RegisterAction("preview.run", func() error {
		called++
		return renderer.SetText("status", "done")
	}); err != nil {
		t.Fatal(err)
	}
	renderer.PointerDown(5, 5)
	if err := renderer.PointerUp(5, 5); err != nil || called != 0 {
		t.Fatalf("disabled top button should block the lower action: called=%d err=%v", called, err)
	}
	renderer.PointerDown(15, 5)
	if err := renderer.PointerUp(15, 5); err != nil || called != 1 {
		t.Fatalf("button action did not run: called=%d err=%v", called, err)
	}
	renderer.PointerDown(15, 5)
	if err := renderer.PointerUp(25, 5); err != nil || called != 1 {
		t.Fatalf("releasing outside the pressed button should cancel the action: called=%d err=%v", called, err)
	}
	second := frameFor(t, renderer, 40, 40)
	if second == first || reflect.DeepEqual(first.Pix, second.Pix) {
		t.Fatal("text changes should repaint and publish a new frame")
	}
	if err := renderer.SetColor("run", blue); err != nil {
		t.Fatal(err)
	}
	third := frameFor(t, renderer, 40, 40)
	if third == second {
		t.Fatal("color changes should publish a new frame")
	}
	pixel(t, third, 15, 5, color.RGBA{B: 255, A: 255})
	pixel(t, third, 9, 31, color.RGBA{B: 255, A: 255})
	if err := renderer.SetVisible("group", false); err != nil {
		t.Fatal(err)
	}
	fourth := frameFor(t, renderer, 40, 40)
	pixel(t, fourth, 9, 31, black)
	pixel(t, fourth, 31, 31, color.RGBA{R: 255, A: 255})
}

func TestUpdateBeforeFirstFrame(t *testing.T) {
	status := textNode(0, 0, 100, 30, "old")
	status.ID = "status"
	renderer := rendererFor(t, status)
	if err := renderer.SetText("status", "ready"); err != nil {
		t.Fatal(err)
	}
	frame := frameFor(t, renderer, 100, 30)
	if inkBounds(frame).Empty() {
		t.Fatal("updates made before the first frame should be drawn")
	}
}
