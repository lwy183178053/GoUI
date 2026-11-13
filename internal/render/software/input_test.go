package software

import (
	"image"
	"image/color"
	"reflect"
	"testing"

	"github.com/lwy183178053/GoUI/internal/ui"
)

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

// 隐藏或禁用应取消整次按压，即使按钮在抬起前已经恢复。
func TestButtonStateChangeCancelsPress(t *testing.T) {
	tests := []struct {
		name         string
		setAvailable func(*Renderer, bool) error
	}{
		{"隐藏按钮", func(r *Renderer, available bool) error { return r.SetVisible("button", available) }},
		{"隐藏父节点", func(r *Renderer, available bool) error { return r.SetVisible("group", available) }},
		{"禁用按钮", func(r *Renderer, available bool) error { return r.SetDisabled("button", !available) }},
	}
	for _, tt := range tests {
		for _, duringMouseUp := range []bool{false, true} {
			phase := "抬起前"
			if duringMouseUp {
				phase = "抬起回调中"
			}
			t.Run(tt.name+"/"+phase, func(t *testing.T) {
				button := block(0, 0, 10, 10, red)
				button.ID, button.Kind = "button", ui.Button
				button.Button = &ui.ButtonStyle{HoverColor: &blue}
				group := block(0, 0, 20, 20, green, button)
				group.ID = "group"
				renderer := rendererFor(t, group)
				frameFor(t, renderer, 20, 20)
				interrupt := func() error {
					if err := tt.setAvailable(renderer, false); err != nil {
						return err
					}
					return tt.setAvailable(renderer, true)
				}
				clicks, enters := 0, 0
				if err := renderer.SetButtonEvent("button", ui.Click, func() error { clicks++; return nil }); err != nil {
					t.Fatal(err)
				}
				if err := renderer.SetButtonEvent("button", ui.MouseEnter, func() error { enters++; return nil }); err != nil {
					t.Fatal(err)
				}
				if duringMouseUp {
					if err := renderer.SetButtonEvent("button", ui.MouseUp, interrupt); err != nil {
						t.Fatal(err)
					}
				}
				if err := renderer.PointerMove(2, 2); err != nil {
					t.Fatal(err)
				}
				if err := renderer.PointerDown(2, 2); err != nil {
					t.Fatal(err)
				}
				if !duringMouseUp {
					if err := interrupt(); err != nil {
						t.Fatal(err)
					}
				}
				if err := renderer.PointerUp(2, 2); err != nil {
					t.Fatal(err)
				}
				if clicks != 0 {
					t.Fatalf("状态变化后仍触发了 %d 次旧点击", clicks)
				}
				if renderer.hovered != nil {
					t.Fatal("状态变化后仍保留旧悬停目标")
				}
				frame := frameFor(t, renderer, 20, 20)
				if got := frame.RGBAAt(2, 2); got.R != 255 || got.B != 0 {
					t.Fatalf("恢复后残留悬停颜色：%v", got)
				}
				if err := renderer.PointerMove(2, 2); err != nil {
					t.Fatal(err)
				}
				if enters != 2 {
					t.Fatalf("恢复后鼠标进入次数 = %d，期望 2", enters)
				}
				if !duringMouseUp {
					if err := renderer.PointerDown(2, 2); err != nil {
						t.Fatal(err)
					}
					if err := renderer.PointerUp(2, 2); err != nil {
						t.Fatal(err)
					}
					if clicks != 1 {
						t.Fatal("恢复后重新按下没有正常触发点击")
					}
				}
			})
		}
	}
}

func TestMouseLeaveCanHideNextTarget(t *testing.T) {
	for _, restore := range []bool{false, true} {
		name := "保持隐藏"
		if restore {
			name = "回调中恢复"
		}
		t.Run(name, func(t *testing.T) {
			first := block(0, 0, 10, 10, red)
			first.ID, first.Kind, first.Button = "first", ui.Button, &ui.ButtonStyle{}
			second := block(10, 0, 10, 10, blue)
			second.ID, second.Kind, second.Button = "second", ui.Button, &ui.ButtonStyle{}
			renderer := rendererFor(t, first, second)
			frameFor(t, renderer, 20, 10)
			if err := renderer.SetButtonEvent("first", ui.MouseLeave, func() error {
				if err := renderer.SetVisible("second", false); err != nil {
					return err
				}
				if restore {
					return renderer.SetVisible("second", true)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			enters := 0
			if err := renderer.SetButtonEvent("second", ui.MouseEnter, func() error { enters++; return nil }); err != nil {
				t.Fatal(err)
			}
			if err := renderer.PointerMove(2, 2); err != nil {
				t.Fatal(err)
			}
			if err := renderer.PointerMove(12, 2); err != nil {
				t.Fatal(err)
			}
			if enters != 0 || renderer.hovered != nil {
				t.Fatal("移出回调隐藏的新目标仍然接收进入事件")
			}
			if restore {
				if err := renderer.PointerMove(12, 2); err != nil {
					t.Fatal(err)
				}
				if enters != 1 || renderer.hovered != second {
					t.Fatal("恢复后的按钮未正常接收进入事件")
				}
			}
		})
	}
}

// 移动使用取整后的矩形绘制，命中也应覆盖完全相同的像素。
func TestTranslatedButtonHitMatchesDrawnPixels(t *testing.T) {
	for _, offset := range []float64{-0.6, -0.5, -0.4, 0.4, 0.5, 0.6} {
		for _, nested := range []bool{false, true} {
			button := block(5, 5, 5, 4, red)
			button.ID, button.Kind, button.Button = "button", ui.Button, &ui.ButtonStyle{}
			root, animatedID := button, "button"
			if nested {
				root = block(1, 1, 16, 16, color.NRGBA{}, button)
				root.ID, animatedID = "group", "group"
			}
			renderer := rendererFor(t, root)
			frameFor(t, renderer, 20, 20)
			position := renderer.nodes[animatedID].Position
			renderer.motions[animatedID] = &animation{
				position: [2]float64{float64(position.X) + offset, float64(position.Y) + offset},
				scale:    [2]float64{1, 1},
			}
			renderer.invalidate(renderer.frame.Bounds())
			frame := frameFor(t, renderer, 20, 20)
			for y := 0; y < 20; y++ {
				for x := 0; x < 20; x++ {
					drawn := frame.RGBAAt(x, y).R == 255
					hit := renderer.hitButton(x, y) == button
					if hit != drawn {
						t.Fatalf("offset=%v nested=%v 像素(%d,%d)：绘制=%v，命中=%v", offset, nested, x, y, drawn, hit)
					}
				}
			}
		}
	}
}

func TestButtonHitOrderAndVisibility(t *testing.T) {
	lower := block(0, 0, 20, 20, red)
	lower.Kind, lower.Button = ui.Button, &ui.ButtonStyle{}
	overlay := block(0, 0, 20, 20, green)
	upper := block(0, 0, 10, 20, blue)
	upper.ID, upper.Kind, upper.Button = "upper", ui.Button, &ui.ButtonStyle{Disabled: true}
	renderer := rendererFor(t, lower, overlay, upper)
	frameFor(t, renderer, 30, 20)
	if got := renderer.hitButton(5, 5); got != upper {
		t.Fatal("最上层禁用按钮没有挡住下层按钮")
	}
	if got := renderer.hitButton(15, 5); got != lower {
		t.Fatal("普通 Block 不应拦截按钮命中")
	}
	if err := renderer.SetVisible("upper", false); err != nil {
		t.Fatal(err)
	}
	if got := renderer.hitButton(5, 5); got != lower {
		t.Fatal("隐藏按钮仍然参与命中")
	}
}
