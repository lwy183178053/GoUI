package software

import (
	"image"
	"image/color"
	"math"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/lwy183178053/GoUI/internal/ui"

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

func TestRoundedMaskHasSmoothCoverageAndRendererCachesIt(t *testing.T) {
	mask := roundedMaskForSize(image.Pt(48, 48), 17.3)
	levels := make(map[uint8]bool)
	for _, alpha := range mask.Pix {
		levels[alpha] = true
	}
	if len(levels) < 12 {
		t.Fatalf("rounded mask has only %d coverage levels, want smoother edges", len(levels))
	}

	renderer := rendererFor(t)
	first := renderer.roundedMask(image.Pt(48, 48), 17.3)
	second := renderer.roundedMask(image.Pt(48, 48), 17.3)
	if first != second {
		t.Fatal("rounded masks should be reused between frames")
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

func TestChangedFramesReusePixelStorage(t *testing.T) {
	target := block(0, 0, 8, 8, red)
	target.ID = "target"
	renderer := rendererFor(t, target)
	first := frameFor(t, renderer, 16, 16)
	pixelStorage := &first.Pix[0]

	if err := renderer.SetColor("target", green); err != nil {
		t.Fatal(err)
	}
	second := frameFor(t, renderer, 16, 16)
	if second == first || &second.Pix[0] != pixelStorage {
		t.Fatal("changed frame should rotate its handle and reuse pixel storage")
	}
	pixel(t, second, 1, 1, color.RGBA{G: 255, A: 255})
	if err := renderer.SetColor("target", blue); err != nil {
		t.Fatal(err)
	}
	third := frameFor(t, renderer, 16, 16)
	if third != first || &third.Pix[0] != pixelStorage {
		t.Fatal("next changed frame did not rotate back to the first handle")
	}
	pixel(t, third, 1, 1, color.RGBA{B: 255, A: 255})
}

func TestScaledImagesAreCached(t *testing.T) {
	node := block(0, 0, 8, 8, color.NRGBA{})
	node.Kind = ui.Image
	node.Image = &ui.ImageStyle{Image: image.NewRGBA(image.Rect(0, 0, 2, 2)), Fit: "stretch"}
	renderer := rendererFor(t, node)

	first := renderer.scaledImage(node, image.Rect(0, 0, 8, 8), node.Image.Image)
	if first == nil || renderer.scaledImage(node, image.Rect(0, 0, 8, 8), node.Image.Image) != first {
		t.Fatal("unchanged image size was not cached")
	}
	if renderer.scaledImage(node, image.Rect(0, 0, 10, 8), node.Image.Image) == first {
		t.Fatal("resizing an image node reused the old scaled image")
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

// 长回答按节点高度选择完整行，底部对齐时应只留下最新内容。
func TestWrappedTextOverflowShowsCompleteLines(t *testing.T) {
	for _, test := range []struct {
		align, want string
	}{
		{"top", "AAAA\nBBBB"},
		{"middle", "CCCC\nDDDD"},
		{"bottom", "EEEE\nFFFF"},
	} {
		t.Run(test.align, func(t *testing.T) {
			rect := image.Rect(0, 0, 80, 31) // 13px 行高，只能容纳两行。
			style := ui.TextStyle{Content: "AAAA\nBBBB\nCCCC\nDDDD\nEEEE\nFFFF", Color: red, Wrap: true, VAlign: test.align}
			got := image.NewRGBA(rect)
			paintText(got, rect, &style, basicfont.Face7x13, rect)
			style.Content = test.want
			want := image.NewRGBA(rect)
			paintText(want, rect, &style, basicfont.Face7x13, rect)
			if inkBounds(got).Empty() || !reflect.DeepEqual(got.Pix, want.Pix) {
				t.Fatal("多行溢出时出现半行文字，或保留了错误的段落")
			}
		})
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
	doc, err := ui.Load(filepath.Join("..", "..", "..", "example", "hello-world", "ui"))
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
	lower.Button = &ui.ButtonStyle{}
	upper := block(0, 0, 10, 20, green)
	upper.ID = "disabled"
	upper.Kind = ui.Button
	upper.Button = &ui.ButtonStyle{Disabled: true}
	status := textNode(20, 0, 20, 20, "old")
	status.ID = "status"
	group := block(0, 22, 5, 5, green, block(8, 8, 4, 4, blue))
	group.ID = "group"
	outside := block(30, 30, 5, 5, red)
	renderer := rendererFor(t, lower, upper, status, group, outside)
	first := frameFor(t, renderer, 40, 40)
	firstPixels := append([]byte(nil), first.Pix...)
	called := 0
	if err := renderer.SetButtonEvent("disabled", ui.Click, func() error { called += 10; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := renderer.SetButtonEvent("run", ui.Click, func() error {
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
	if err := renderer.PointerUp(5, 5); err != nil || called != 1 {
		t.Fatalf("releasing over another button should cancel the action: called=%d err=%v", called, err)
	}
	renderer.PointerDown(15, 5)
	if err := renderer.PointerUp(25, 5); err != nil || called != 1 {
		t.Fatalf("releasing outside the pressed button should cancel the action: called=%d err=%v", called, err)
	}
	second := frameFor(t, renderer, 40, 40)
	if second == first || reflect.DeepEqual(firstPixels, second.Pix) {
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

func TestButtonMouseEventsFollowPointer(t *testing.T) {
	button := block(0, 0, 10, 10, red)
	button.ID = "button"
	button.Kind = ui.Button
	button.Button = &ui.ButtonStyle{}
	renderer := rendererFor(t, button)
	frameFor(t, renderer, 20, 20)

	var events []ui.ButtonEvent
	for _, event := range []ui.ButtonEvent{ui.MouseEnter, ui.MouseLeave, ui.MouseDown, ui.MouseUp, ui.Click} {
		event := event
		if err := renderer.SetButtonEvent("button", event, func() error {
			events = append(events, event)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	for _, step := range []struct {
		move image.Point
		down bool
		up   bool
	}{
		{move: image.Pt(2, 2)},
		{move: image.Pt(3, 3)},
		{move: image.Pt(3, 3), down: true},
		{move: image.Pt(15, 15)},
		{move: image.Pt(2, 2)},
		{move: image.Pt(2, 2), up: true},
	} {
		if err := renderer.PointerMove(step.move.X, step.move.Y); err != nil {
			t.Fatal(err)
		}
		if step.down {
			if err := renderer.PointerDown(step.move.X, step.move.Y); err != nil {
				t.Fatal(err)
			}
		}
		if step.up {
			if err := renderer.PointerUp(step.move.X, step.move.Y); err != nil {
				t.Fatal(err)
			}
		}
	}
	want := []ui.ButtonEvent{ui.MouseEnter, ui.MouseDown, ui.MouseLeave, ui.MouseEnter, ui.MouseUp, ui.Click}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("button events = %v, want %v", events, want)
	}
}

func TestButtonHoverColorChangesAndRestoresAutomatically(t *testing.T) {
	base := color.NRGBA{R: 32, G: 64, B: 96, A: 255}
	hover := color.NRGBA{R: 96, G: 160, B: 224, A: 255}
	button := block(2, 2, 10, 10, base)
	button.ID = "button"
	button.Kind = ui.Button
	button.Button = &ui.ButtonStyle{HoverColor: &hover}
	renderer := rendererFor(t, button)
	frameFor(t, renderer, 16, 16)

	if err := renderer.PointerMove(4, 4); err != nil {
		t.Fatal(err)
	}
	pixel(t, frameFor(t, renderer, 16, 16), 4, 4, color.RGBA{R: 96, G: 160, B: 224, A: 255})

	if err := renderer.SetColor("button", blue); err != nil {
		t.Fatal(err)
	}
	pixel(t, frameFor(t, renderer, 16, 16), 4, 4, color.RGBA{R: 96, G: 160, B: 224, A: 255})

	if err := renderer.PointerMove(15, 15); err != nil {
		t.Fatal(err)
	}
	pixel(t, frameFor(t, renderer, 16, 16), 4, 4, color.RGBA{B: 255, A: 255})
}

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
